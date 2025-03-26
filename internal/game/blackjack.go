package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type GameStatus string

const (
	Waiting    GameStatus = "waiting"    // Waiting for players to join
	Betting    GameStatus = "betting"    // Players are placing bets
	InProgress GameStatus = "inProgress" // Game is in progress
	Completed  GameStatus = "completed"  // Game is completed
)

type PlayerStatus string

const (
	PlayerActive    PlayerStatus = "active"    // Player is still in the game
	PlayerBusted    PlayerStatus = "busted"    // Player busted (score > 21)
	PlayerStood     PlayerStatus = "stood"     // Player decided to stand
	PlayerBlackjack PlayerStatus = "blackjack" // Player has blackjack
)

type Player struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Hand     []Card       `json:"hand"`
	Score    int          `json:"score"`
	Status   PlayerStatus `json:"status"`
	Bet      int          `json:"bet"`
	Balance  int          `json:"balance"`
	IsActive bool         `json:"isActive"` // True if it's this player's turn
}

type Dealer struct {
	Hand  []Card `json:"hand"`
	Score int    `json:"score"`
}

type BlackjackGame struct {
	ID                 string     `json:"id"`
	Players            []Player   `json:"players"`
	Dealer             Dealer     `json:"dealer"`
	Deck               *Deck      `json:"deck,omitempty"`
	Status             GameStatus `json:"status"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	MinBet             int        `json:"minBet"`
	MaxBet             int        `json:"maxBet"`
	TableID            string     `json:"tableId"`
	CurrentPlayerIndex int        `json:"currentPlayerIndex"`
}

// NewBlackjackGame creates a new blackjack game
func NewBlackjackGame(tableID string, minBet, maxBet int) *BlackjackGame {
	deck := NewDeck()
	deck.Shuffle()

	now := time.Now()

	return &BlackjackGame{
		ID:                 uuid.New().String(),
		Players:            []Player{},
		Dealer:             Dealer{Hand: []Card{}, Score: 0},
		Deck:               deck,
		Status:             Waiting,
		CreatedAt:          now,
		UpdatedAt:          now,
		MinBet:             minBet,
		MaxBet:             maxBet,
		TableID:            tableID,
		CurrentPlayerIndex: 0,
	}
}

// AddPlayer adds a player to the game
func (g *BlackjackGame) AddPlayer(playerID, playerName string, initialBalance int) *Player {
	// Check if player is already in the game
	for i, p := range g.Players {
		if p.ID == playerID {
			// Player exists, update their status if needed
			if g.Status == Waiting {
				g.Players[i].IsActive = true
				return &g.Players[i]
			}
			return &g.Players[i]
		}
	}

	fmt.Println("Adding player", playerName)
	fmt.Println("Initial balance", initialBalance)
	fmt.Println("Player ID", playerID)
	fmt.Println("g.Status", g.Status)
	fmt.Println("g.Players", g.Players)
	// If the game is already in progress, don't add new players
	if g.Status != Waiting {
		return nil
	}

	// Add new player
	player := Player{
		ID:       playerID,
		Name:     playerName,
		Hand:     []Card{},
		Score:    0,
		Status:   PlayerActive,
		Bet:      0,
		Balance:  initialBalance,
		IsActive: false,
	}

	g.Players = append(g.Players, player)
	g.UpdatedAt = time.Now()

	return &player
}

// RemovePlayer removes a player from the game
func (g *BlackjackGame) RemovePlayer(playerID string) bool {
	for i, p := range g.Players {
		if p.ID == playerID {
			// Remove player from slice
			g.Players = append(g.Players[:i], g.Players[i+1:]...)
			g.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// PlaceBet allows a player to place a bet
func (g *BlackjackGame) PlaceBet(playerID string, amount int) bool {
	if g.Status != Betting {
		return false
	}

	// Validate bet amount
	if amount < g.MinBet || amount > g.MaxBet {
		return false
	}

	for i, p := range g.Players {
		if p.ID == playerID {
			// Check if player has enough balance
			if p.Balance < amount {
				return false
			}

			// Place the bet
			g.Players[i].Bet = amount
			g.Players[i].Balance -= amount
			g.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// Start begins the game after all players have placed their bets
func (g *BlackjackGame) Start() bool {
	if g.Status != Betting || len(g.Players) == 0 {
		return false
	}

	// Check if all players have placed bets
	for _, p := range g.Players {
		if p.Bet == 0 {
			return false
		}
	}

	// Deal initial cards
	g.DealInitialCards()

	// Set game status to in progress
	g.Status = InProgress
	g.UpdatedAt = time.Now()

	// Set current player
	g.CurrentPlayerIndex = 0
	g.Players[0].IsActive = true

	return true
}

// DealInitialCards deals the initial cards to all players and the dealer
func (g *BlackjackGame) DealInitialCards() {
	// Deal two cards to each player
	for i := range g.Players {
		// First card face up
		card1, _ := g.Deck.DrawCard()
		card1.Face = true
		g.Players[i].Hand = append(g.Players[i].Hand, card1)

		// Second card face up
		card2, _ := g.Deck.DrawCard()
		card2.Face = true
		g.Players[i].Hand = append(g.Players[i].Hand, card2)

		// Calculate initial score
		g.Players[i].Score = g.CalculateHandScore(g.Players[i].Hand)

		// Check for blackjack
		if g.Players[i].Score == 21 {
			g.Players[i].Status = PlayerBlackjack
		}
	}

	// Deal two cards to dealer, first face up, second face down
	dealerCard1, _ := g.Deck.DrawCard()
	dealerCard1.Face = true
	g.Dealer.Hand = append(g.Dealer.Hand, dealerCard1)

	dealerCard2, _ := g.Deck.DrawCard()
	dealerCard2.Face = false // Dealer's second card is face down
	g.Dealer.Hand = append(g.Dealer.Hand, dealerCard2)

	// Calculate dealer's visible score (only count face-up cards)
	g.Dealer.Score = dealerCard1.GetValue()
}

// Hit gives the current player another card
func (g *BlackjackGame) Hit(playerID string) (Card, bool) {
	if g.Status != InProgress {
		return Card{}, false
	}

	// Find player
	for i, p := range g.Players {
		if p.ID == playerID && p.IsActive && p.Status == PlayerActive {
			// Draw a card
			card, success := g.Deck.DrawCard()
			if !success {
				return Card{}, false
			}

			card.Face = true
			g.Players[i].Hand = append(g.Players[i].Hand, card)

			// Recalculate score
			g.Players[i].Score = g.CalculateHandScore(g.Players[i].Hand)

			// Check if busted
			if g.Players[i].Score > 21 {
				g.Players[i].Status = PlayerBusted
				g.Players[i].IsActive = false
				g.NextPlayer()
			}

			g.UpdatedAt = time.Now()
			return card, true
		}
	}
	return Card{}, false
}

// Stand ends the current player's turn
func (g *BlackjackGame) Stand(playerID string) bool {
	if g.Status != InProgress {
		return false
	}

	// Find player
	for i, p := range g.Players {
		if p.ID == playerID && p.IsActive && p.Status == PlayerActive {
			g.Players[i].Status = PlayerStood
			g.Players[i].IsActive = false

			g.NextPlayer()
			g.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// NextPlayer moves to the next player or dealer's turn if all players are done
func (g *BlackjackGame) NextPlayer() {
	// Find next active player
	nextIndex := (g.CurrentPlayerIndex + 1) % len(g.Players)
	startIndex := nextIndex

	for {
		if g.Players[nextIndex].Status == PlayerActive {
			g.CurrentPlayerIndex = nextIndex
			g.Players[nextIndex].IsActive = true
			return
		}

		nextIndex = (nextIndex + 1) % len(g.Players)

		// If we've checked all players and none are active, it's dealer's turn
		if nextIndex == startIndex {
			g.DealerTurn()
			return
		}
	}
}

// DealerTurn plays the dealer's turn after all players have played
func (g *BlackjackGame) DealerTurn() {
	// Flip the dealer's face-down card
	for i := range g.Dealer.Hand {
		g.Dealer.Hand[i].Face = true
	}

	// Calculate dealer's score with all cards
	g.Dealer.Score = g.CalculateHandScore(g.Dealer.Hand)

	// Dealer must draw until score is at least 17
	for g.Dealer.Score < 17 {
		card, success := g.Deck.DrawCard()
		if !success {
			break
		}

		card.Face = true
		g.Dealer.Hand = append(g.Dealer.Hand, card)
		g.Dealer.Score = g.CalculateHandScore(g.Dealer.Hand)
	}

	// Determine winners and pay out
	g.DetermineWinners()

	// Game is completed
	g.Status = Completed
	g.UpdatedAt = time.Now()
}

// DetermineWinners determines winners and updates player balances
func (g *BlackjackGame) DetermineWinners() {
	dealerScore := g.Dealer.Score
	dealerBusted := dealerScore > 21

	for i, player := range g.Players {
		playerScore := player.Score

		switch player.Status {
		case PlayerBusted:
			// Player busted, they lose
			continue

		case PlayerBlackjack:
			// Player has blackjack, pays 3:2 unless dealer also has blackjack
			if len(g.Dealer.Hand) == 2 && dealerScore == 21 {
				// Push - both have blackjack
				g.Players[i].Balance += player.Bet
			} else {
				// Player wins with blackjack
				g.Players[i].Balance += player.Bet + int(float64(player.Bet)*1.5)
			}

		default:
			// Normal win/loss/push
			if dealerBusted {
				// Dealer busted, player wins
				g.Players[i].Balance += player.Bet * 2
			} else if playerScore > dealerScore {
				// Player score higher than dealer
				g.Players[i].Balance += player.Bet * 2
			} else if playerScore == dealerScore {
				// Push
				g.Players[i].Balance += player.Bet
			}
			// Otherwise dealer wins, player already lost their bet
		}
	}
}

// CalculateHandScore calculates the score of a hand, accounting for aces
func (g *BlackjackGame) CalculateHandScore(hand []Card) int {
	score := 0
	aces := 0

	// First pass: calculate score treating aces as 11
	for _, card := range hand {
		if card.Rank == Ace {
			aces++
		}
		score += card.GetValue()
	}

	// Second pass: convert aces from 11 to 1 as needed to avoid busting
	for aces > 0 && score > 21 {
		score -= 10 // Convert one ace from 11 to 1 (11 - 10 = 1)
		aces--
	}

	return score
}

// PrepareForNextRound resets the game for a new round while keeping player balances
func (g *BlackjackGame) PrepareForNextRound() {
	// Create a new deck and shuffle
	g.Deck = NewDeck()
	g.Deck.Shuffle()

	// Reset dealer
	g.Dealer.Hand = []Card{}
	g.Dealer.Score = 0

	// Reset players but keep their balances
	for i := range g.Players {
		g.Players[i].Hand = []Card{}
		g.Players[i].Score = 0
		g.Players[i].Status = PlayerActive
		g.Players[i].Bet = 0
		g.Players[i].IsActive = false
	}

	// Set game status to betting
	g.Status = Betting
	g.UpdatedAt = time.Now()
}

// GetGameState returns the current game state
func (g *BlackjackGame) GetGameState(playerID string) map[string]interface{} {
	gameState := map[string]interface{}{
		"id":      g.ID,
		"status":  g.Status,
		"dealer":  g.Dealer,
		"tableId": g.TableID,
		"minBet":  g.MinBet,
		"maxBet":  g.MaxBet,
	}

	// Include sanitized player data for all players
	sanitizedPlayers := make([]map[string]interface{}, len(g.Players))
	for i, player := range g.Players {
		sanitizedPlayer := map[string]interface{}{
			"id":       player.ID,
			"name":     player.Name,
			"score":    player.Score,
			"status":   player.Status,
			"bet":      player.Bet,
			"isActive": player.IsActive,
		}

		// Only include sensitive data for the current player
		if player.ID == playerID {
			sanitizedPlayer["hand"] = player.Hand
			sanitizedPlayer["balance"] = player.Balance
		} else {
			sanitizedPlayer["hand"] = player.Hand
		}

		sanitizedPlayers[i] = sanitizedPlayer
	}

	gameState["players"] = sanitizedPlayers

	return gameState
}
