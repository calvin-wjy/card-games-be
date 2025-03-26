package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/calvinwijaya/card-games-be/internal/db"
	"github.com/calvinwijaya/card-games-be/internal/game"
	"github.com/calvinwijaya/card-games-be/internal/store"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Handlers contains all the API handlers
type Handlers struct {
	store    store.Store
	database *db.Database
	hub      *Hub
}

// NewHandlers creates a new instance of Handlers
func NewHandlers(store store.Store, database *db.Database, hub *Hub) *Handlers {
	return &Handlers{
		store:    store,
		database: database,
		hub:      hub,
	}
}

// RegisterRoutes registers all API routes
func (h *Handlers) RegisterRoutes(r *mux.Router) {
	// Game endpoints
	r.HandleFunc("/api/game/new", h.NewGame).Methods("POST")
	r.HandleFunc("/api/game/{id}/hit", h.Hit).Methods("POST")
	r.HandleFunc("/api/game/{id}/stand", h.Stand).Methods("POST")
	r.HandleFunc("/api/game/{id}/bet", h.PlaceBet).Methods("POST")
	r.HandleFunc("/api/game/{id}", h.GetGame).Methods("GET")

	// Player endpoints
	r.HandleFunc("/api/player/register", h.RegisterPlayer).Methods("POST")
	r.HandleFunc("/api/player/{id}", h.GetPlayer).Methods("GET")
	r.HandleFunc("/api/player/{id}/stats", h.GetPlayerStats).Methods("GET")

	// Table endpoints
	r.HandleFunc("/api/table/list", h.ListTables).Methods("GET")
	r.HandleFunc("/api/table/{id}/join", h.JoinTable).Methods("POST")
	r.HandleFunc("/api/table/{id}/leave", h.LeaveTable).Methods("POST")

	// WebSocket endpoint
	r.HandleFunc("/ws", h.hub.WebSocketHandler)
}

// response helper function to send JSON responses
func response(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// error response helper function
func errorResponse(w http.ResponseWriter, status int, message string) {
	response(w, status, map[string]string{"error": message})
}

// NewGame creates a new blackjack game
func (h *Handlers) NewGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableID string `json:"tableId"`
		MinBet  int    `json:"minBet"`
		MaxBet  int    `json:"maxBet"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TableID == "" {
		req.TableID = uuid.New().String()
	}

	// Set default bet limits if not provided
	if req.MinBet <= 0 {
		req.MinBet = 10
	}
	if req.MaxBet <= 0 || req.MaxBet < req.MinBet {
		req.MaxBet = req.MinBet * 100
	}

	// Create a new game
	g := game.NewBlackjackGame(req.TableID, req.MinBet, req.MaxBet)

	// Change status to betting phase
	// g.Status = game.Betting

	// Save to store
	if err := h.store.SaveGame(g); err != nil {
		fmt.Println("err: ", err)
		errorResponse(w, http.StatusInternalServerError, "Failed to save game")
		return
	}

	// Save to database if available
	if h.database != nil {
		if err := h.database.SaveGame(g); err != nil {
			// Log but don't fail the request
			// We can recover this later
		}
	}

	// Broadcast game creation to the table
	if h.hub != nil {
		h.hub.BroadcastToTable(g.TableID, Message{
			Type:    "gameCreated",
			GameID:  g.ID,
			TableID: g.TableID,
			Data:    g,
		})
	}

	response(w, http.StatusCreated, g)
}

// Hit allows a player to take another card
func (h *Handlers) Hit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	var req struct {
		PlayerID string `json:"playerId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the game from store
	g, err := h.store.GetGame(gameID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Game not found")
		return
	}

	// Perform hit action
	card, success := g.Hit(req.PlayerID)
	if !success {
		errorResponse(w, http.StatusBadRequest, "Unable to hit")
		return
	}

	// Update game in store
	if err := h.store.SaveGame(g); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to update game")
		return
	}

	// Broadcast game update to all players
	if h.hub != nil {
		h.hub.BroadcastGameUpdate(g)
	}

	response(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"card":    card,
		"game":    g.GetGameState(req.PlayerID),
	})
}

// Stand allows a player to end their turn
func (h *Handlers) Stand(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	var req struct {
		PlayerID string `json:"playerId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the game from store
	g, err := h.store.GetGame(gameID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Game not found")
		return
	}

	// Perform stand action
	if success := g.Stand(req.PlayerID); !success {
		errorResponse(w, http.StatusBadRequest, "Unable to stand")
		return
	}

	// Update game in store
	if err := h.store.SaveGame(g); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to update game")
		return
	}

	// Broadcast game update to all players
	if h.hub != nil {
		h.hub.BroadcastGameUpdate(g)
	}

	// If game is completed, save results to database
	if g.Status == game.Completed && h.database != nil {
		// Update game status in database
		h.database.UpdateGameStatus(g.ID, g.Status)

		// Save game results for each player
		for _, player := range g.Players {
			var result string
			var winnings int

			if player.Status == game.PlayerBusted {
				result = "lose"
				winnings = 0
			} else if player.Status == game.PlayerBlackjack {
				result = "blackjack"
				// Blackjack pays 3:2
				winnings = player.Bet + int(float64(player.Bet)*1.5)
			} else {
				dealerScore := g.Dealer.Score
				playerScore := player.Score

				if dealerScore > 21 || playerScore > dealerScore {
					result = "win"
					winnings = player.Bet * 2
				} else if playerScore == dealerScore {
					result = "push"
					winnings = player.Bet
				} else {
					result = "lose"
					winnings = 0
				}
			}

			h.database.SaveGameResult(g.ID, player.ID, player.Bet, result, winnings)

			// Update player balance in database
			h.database.UpdatePlayerBalance(player.ID, player.Balance)
		}
	}

	response(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"game":    g.GetGameState(req.PlayerID),
	})
}

// PlaceBet allows a player to place a bet
func (h *Handlers) PlaceBet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	var req struct {
		PlayerID string `json:"playerId"`
		Amount   int    `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the game from store
	g, err := h.store.GetGame(gameID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Game not found")
		return
	}

	// Place the bet
	if success := g.PlaceBet(req.PlayerID, req.Amount); !success {
		errorResponse(w, http.StatusBadRequest, "Unable to place bet")
		return
	}

	// Update game in store
	if err := h.store.SaveGame(g); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to update game")
		return
	}

	// Broadcast game update to all players
	if h.hub != nil {
		h.hub.BroadcastGameUpdate(g)
	}

	response(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"game":    g.GetGameState(req.PlayerID),
	})
}

// GetGame returns the current state of a game
func (h *Handlers) GetGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	playerID := r.URL.Query().Get("playerId")

	// Get the game from store
	g, err := h.store.GetGame(gameID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Game not found")
		return
	}

	// Return the game state
	response(w, http.StatusOK, g.GetGameState(playerID))
}

// RegisterPlayer registers a new player
func (h *Handlers) RegisterPlayer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		UserID string `json:"userId,omitempty"` // External user ID if you have authentication
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		errorResponse(w, http.StatusBadRequest, "Player name is required")
		return
	}

	// Generate a player ID
	playerID := uuid.New().String()
	initialBalance := 1000 // Default starting balance

	// Create player in database if available
	if h.database != nil {
		if err := h.database.CreatePlayer(playerID, req.Name, initialBalance); err != nil {
			errorResponse(w, http.StatusInternalServerError, "Failed to create player")
			return
		}
	}

	response(w, http.StatusCreated, map[string]interface{}{
		"id":      playerID,
		"name":    req.Name,
		"balance": initialBalance,
	})
}

// GetPlayer returns player information
func (h *Handlers) GetPlayer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]

	if h.database == nil {
		errorResponse(w, http.StatusInternalServerError, "Database not available")
		return
	}

	// Get player from database
	player, err := h.database.GetPlayerByID(playerID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Error retrieving player")
		return
	}

	if player == nil {
		errorResponse(w, http.StatusNotFound, "Player not found")
		return
	}

	// Update last login time
	h.database.UpdatePlayerLastLogin(playerID)

	response(w, http.StatusOK, player)
}

// GetPlayerStats returns player statistics
func (h *Handlers) GetPlayerStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]

	if h.database == nil {
		errorResponse(w, http.StatusInternalServerError, "Database not available")
		return
	}

	// Get player stats from database
	stats, err := h.database.GetPlayerStats(playerID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Error retrieving player statistics")
		return
	}

	response(w, http.StatusOK, stats)
}

// JoinTable allows a player to join a table
func (h *Handlers) JoinTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableID := vars["id"]

	var req struct {
		PlayerID   string `json:"playerId"`
		PlayerName string `json:"playerName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get active game for this table
	g, err := h.store.GetActiveTableGame(tableID)
	if err != nil {
		// No active game for this table, create a new one
		g = game.NewBlackjackGame(tableID, 10, 1000) // Default min/max bets
		g.Status = game.Waiting
		h.store.SaveGame(g)
	}

	// If the game is in the Completed state, start a new round
	if g.Status == game.Completed {
		g.PrepareForNextRound()
		h.store.SaveGame(g)
	}

	// Get player from database if available
	var initialBalance int = 1000

	if h.database != nil {
		dbPlayer, err := h.database.GetPlayerByID(req.PlayerID)
		if err == nil && dbPlayer != nil {
			initialBalance = dbPlayer.Balance
		}
	}

	// Add player to the game
	player := g.AddPlayer(req.PlayerID, req.PlayerName, initialBalance)
	if player == nil {
		errorResponse(w, http.StatusBadRequest, "Unable to join table")
		return
	}

	// Save game to store
	if err := h.store.SaveGame(g); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to update game")
		return
	}

	// Broadcast player joined to all players in the table
	if h.hub != nil {
		h.hub.BroadcastToTable(tableID, Message{
			Type:     "playerJoined",
			TableID:  tableID,
			PlayerID: req.PlayerID,
			Data:     player,
		})
	}

	response(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"player":  player,
		"game":    g.GetGameState(req.PlayerID),
	})
}

// LeaveTable allows a player to leave a table
func (h *Handlers) LeaveTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tableID := vars["id"]

	var req struct {
		PlayerID string `json:"playerId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get active game for this table
	g, err := h.store.GetActiveTableGame(tableID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "No active game found for table")
		return
	}

	// Remove player from game
	if !g.RemovePlayer(req.PlayerID) {
		errorResponse(w, http.StatusBadRequest, "Player not found in game")
		return
	}

	// If this was the last player, mark the game as completed
	if len(g.Players) == 0 {
		g.Status = game.Completed
	}

	// Save game to store
	if err := h.store.SaveGame(g); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to update game")
		return
	}

	// Broadcast player left to all players in the table
	if h.hub != nil {
		h.hub.BroadcastToTable(tableID, Message{
			Type:     "playerLeft",
			TableID:  tableID,
			PlayerID: req.PlayerID,
		})
	}

	response(w, http.StatusOK, map[string]string{
		"success": "true",
		"message": "Successfully left table",
	})
}

// ListTables returns a list of available tables
func (h *Handlers) ListTables(w http.ResponseWriter, r *http.Request) {
	// Get all games
	allGames, err := h.store.GetAllGames()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Error retrieving tables")
		return
	}

	// Extract unique table IDs and their info
	tables := make(map[string]map[string]interface{})

	for _, g := range allGames {
		// Skip completed games if there's a newer active game for the table
		if activeGame, _ := h.store.GetActiveTableGame(g.TableID); activeGame != nil && activeGame.ID != g.ID {
			continue
		}

		tables[g.TableID] = map[string]interface{}{
			"id":          g.TableID,
			"playerCount": len(g.Players),
			"status":      g.Status,
			"minBet":      g.MinBet,
			"maxBet":      g.MaxBet,
			"currentGame": g.ID,
			"lastUpdated": g.UpdatedAt.Format(time.RFC3339),
		}
	}

	fmt.Println(tables)

	// Convert map to slice for response
	tablesList := make([]map[string]interface{}, 0, len(tables))
	for _, tableInfo := range tables {
		tablesList = append(tablesList, tableInfo)
	}

	response(w, http.StatusOK, tablesList)
}
