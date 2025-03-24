package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/calvinwijaya/card-games-be/internal/game"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now, customize this in production
	},
}

// Message represents a WebSocket message
type Message struct {
	Type     string      `json:"type"`
	GameID   string      `json:"gameId,omitempty"`
	TableID  string      `json:"tableId,omitempty"`
	PlayerID string      `json:"playerId,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// Client represents a connected WebSocket client
type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	tableID  string
	playerID string
	hub      *Hub
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	tables     map[string]map[*Client]bool
	playerMap  map[string]*Client
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		tables:     make(map[string]map[*Client]bool),
		playerMap:  make(map[string]*Client),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true

			// Add to table map
			if client.tableID != "" {
				if _, exists := h.tables[client.tableID]; !exists {
					h.tables[client.tableID] = make(map[*Client]bool)
				}
				h.tables[client.tableID][client] = true
			}

			// Add to player map
			if client.playerID != "" {
				h.playerMap[client.playerID] = client
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)

				// Remove from table map
				if client.tableID != "" && h.tables[client.tableID] != nil {
					delete(h.tables[client.tableID], client)
					// Clean up empty tables
					if len(h.tables[client.tableID]) == 0 {
						delete(h.tables, client.tableID)
					}
				}

				// Remove from player map
				if client.playerID != "" {
					delete(h.playerMap, client.playerID)
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.mu.Lock()
					close(client.send)
					delete(h.clients, client)
					if client.tableID != "" && h.tables[client.tableID] != nil {
						delete(h.tables[client.tableID], client)
					}
					if client.playerID != "" {
						delete(h.playerMap, client.playerID)
					}
					h.mu.Unlock()
				}
			}
		}
	}
}

// BroadcastToTable sends a message to all clients in a specific table
func (h *Hub) BroadcastToTable(tableID string, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Send to all clients in the table
	if tableClients, exists := h.tables[tableID]; exists {
		for client := range tableClients {
			select {
			case client.send <- data:
			default:
				// If client buffer is full, we'll handle on next write
			}
		}
	}
}

// BroadcastGameUpdate broadcasts a game update to all clients in the table
func (h *Hub) BroadcastGameUpdate(game *game.BlackjackGame) {
	// Send a sanitized game state to all players in the table
	// Each player will receive a customized view with their own data
	h.mu.RLock()
	defer h.mu.RUnlock()

	tableClients, exists := h.tables[game.TableID]
	if !exists {
		return
	}

	for client := range tableClients {
		// Create a customized game state for this player
		gameState := game.GetGameState(client.playerID)

		msg := Message{
			Type:    "gameUpdate",
			GameID:  game.ID,
			TableID: game.TableID,
			Data:    gameState,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling game update: %v", err)
			continue
		}

		select {
		case client.send <- data:
		default:
			// If client buffer is full, we'll handle on next write
		}
	}
}

// SendToPlayer sends a message to a specific player
func (h *Hub) SendToPlayer(playerID string, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	client, exists := h.playerMap[playerID]
	if !exists {
		return
	}

	select {
	case client.send <- data:
	default:
		// If client buffer is full, we'll handle on next write
	}
}

// WebSocketHandler handles WebSocket connections
func (h *Hub) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Extract playerID and tableID from query params
	playerID := r.URL.Query().Get("playerId")
	tableID := r.URL.Query().Get("tableId")

	client := &Client{
		conn:     conn,
		send:     make(chan []byte, 256),
		tableID:  tableID,
		playerID: playerID,
		hub:      h,
	}
	h.register <- client

	// Send a welcome message
	welcomeMsg := Message{
		Type: "welcome",
		Data: map[string]string{
			"message":  "Connected to BlackJack game server",
			"playerId": playerID,
			"tableId":  tableID,
		},
	}
	welcomeData, _ := json.Marshal(welcomeMsg)
	client.send <- welcomeData

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse the message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Process message based on type
		// This will be handled by the API handler
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
