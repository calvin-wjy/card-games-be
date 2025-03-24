# Blackjack Game Backend

This is the Go backend for a multiplayer Blackjack game. It provides a RESTful API and WebSocket support for real-time game updates.

## Features

- RESTful API for game actions (hit, stand, bet)
- WebSocket support for real-time game updates
- Database persistence for player stats and game history
- Multiple table support for concurrent games
- Player authentication and session management

## Prerequisites

- Go 1.21 or higher
- SQLite (for persistence)

## Installation

1. Clone the repository:

```bash
git clone <repository-url>
cd card-games-be
```

2. Install dependencies:

```bash
go mod download
```

3. Build the application:

```bash
go build -o blackjack-server ./cmd/server
```

## Running the Server

```bash
# Basic usage
./blackjack-server

# With custom port
./blackjack-server -port 8888

# With custom database path
./blackjack-server -db ./data/my-blackjack.db

# With custom frontend URL for CORS
./blackjack-server -frontend http://localhost:3000
```

By default, the server runs on port 8080, uses `./data/blackjack.db` for the database, and allows CORS for `http://localhost:5173`.

## API Endpoints

### Game Endpoints

- `POST /api/game/new`: Create a new game
- `POST /api/game/{id}/hit`: Draw a card
- `POST /api/game/{id}/stand`: Stand (end turn)
- `POST /api/game/{id}/bet`: Place a bet
- `GET /api/game/{id}`: Get game state

### Player Endpoints

- `POST /api/player/register`: Register a new player
- `GET /api/player/{id}`: Get player information
- `GET /api/player/{id}/stats`: Get player statistics

### Table Endpoints

- `GET /api/table/list`: List available tables
- `POST /api/table/{id}/join`: Join a table
- `POST /api/table/{id}/leave`: Leave a table

### WebSocket

- `GET /ws?playerId={playerId}&tableId={tableId}`: WebSocket connection

## WebSocket Messages

### Server to Client

- `welcome`: Connection established
- `gameUpdate`: Game state updated
- `playerJoined`: A player joined the table
- `playerLeft`: A player left the table
- `gameCreated`: A new game was created

### Client to Server

- `joinTable`: Join a table
- `leaveTable`: Leave a table
- `placeBet`: Place a bet
- `hit`: Draw a card
- `stand`: End turn

## Development

### Project Structure

```
card-games-be/
├── cmd/
│   └── server/
│       └── main.go       # Entry point for the application
├── internal/
│   ├── api/
│   │   ├── handlers.go   # HTTP handlers
│   │   └── websocket.go  # WebSocket handlers
│   ├── game/
│   │   ├── card.go       # Card model
│   │   ├── deck.go       # Deck model
│   │   └── blackjack.go  # Game logic
│   ├── db/
│   │   └── database.go   # Database interaction
│   └── store/
│       └── memory.go     # In-memory game storage
├── go.mod
└── go.sum
```

### Running Tests

```bash
go test ./...
```

## License

MIT 