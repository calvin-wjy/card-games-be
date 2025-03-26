package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/calvinwijaya/card-games-be/internal/game"
	_ "github.com/lib/pq"
)

type Database struct {
	db *sql.DB
}

type PlayerStats struct {
	PlayerID      string    `json:"playerId"`
	PlayerName    string    `json:"playerName"`
	GamesPlayed   int       `json:"gamesPlayed"`
	GamesWon      int       `json:"gamesWon"`
	TotalBets     int       `json:"totalBets"`
	TotalWinnings int       `json:"totalWinnings"`
	LastPlayed    time.Time `json:"lastPlayed"`
}

// NewDatabase creates a new database connection
func NewDatabase() (*Database, error) {
	// Get database connection details from environment variables
	// dbHost := os.Getenv("DB_HOST")
	// dbPort := os.Getenv("DB_PORT")
	// dbName := os.Getenv("DB_NAME")
	// dbUser := os.Getenv("DB_USER")
	// dbPassword := os.Getenv("DB_PASSWORD")

	// TODO: Remove hardcoded values
	dbHost := "localhost"
	dbPort := "5433"
	dbName := "card_games"
	dbUser := "card_games_user"
	dbPassword := "card_games_password"

	// Construct connection string
	connStr := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		dbHost, dbPort, dbName, dbUser, dbPassword,
	)

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	// Set connection parameters
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Initialize database tables
	if err := initTables(db); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// initTables creates the necessary tables if they don't exist
func initTables(db *sql.DB) error {
	// Players table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			balance INTEGER NOT NULL DEFAULT 1000,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating players table: %v", err)
	}

	// Games table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			table_id TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP,
			status TEXT NOT NULL,
			min_bet INTEGER NOT NULL DEFAULT 10,
			max_bet INTEGER NOT NULL DEFAULT 1000,
			game_state JSONB
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating games table: %v", err)
	}

	// Game results table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS game_results (
			id SERIAL PRIMARY KEY,
			game_id TEXT NOT NULL,
			player_id TEXT NOT NULL,
			bet INTEGER NOT NULL,
			result TEXT NOT NULL,
			winnings INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (game_id) REFERENCES games (id),
			FOREIGN KEY (player_id) REFERENCES players (id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating game_results table: %v", err)
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// GetPlayerByID retrieves a player from the database by ID
func (d *Database) GetPlayerByID(playerID string) (*game.Player, error) {
	var player game.Player
	var balanceInt int
	var lastLogin time.Time

	err := d.db.QueryRow("SELECT id, name, balance, last_login FROM players WHERE id = ?", playerID).Scan(
		&player.ID,
		&player.Name,
		&balanceInt,
		&lastLogin,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Player not found
		}
		return nil, err
	}

	player.Balance = balanceInt
	player.Hand = []game.Card{}
	player.Score = 0
	player.Status = game.PlayerActive
	player.Bet = 0
	player.IsActive = false

	return &player, nil
}

// CreatePlayer creates a new player in the database
func (d *Database) CreatePlayer(playerID, playerName string, initialBalance int) error {
	now := time.Now()
	_, err := d.db.Exec(
		"INSERT INTO players (id, name, balance, created_at, last_login) VALUES ($1, $2, $3, $4, $5)",
		playerID, playerName, initialBalance, now, now,
	)
	return err
}

// UpdatePlayerBalance updates a player's balance in the database
func (d *Database) UpdatePlayerBalance(playerID string, newBalance int) error {
	_, err := d.db.Exec(
		"UPDATE players SET balance = ?, last_login = ? WHERE id = ?",
		newBalance, time.Now(), playerID,
	)
	return err
}

// UpdatePlayerLastLogin updates a player's last login timestamp
func (d *Database) UpdatePlayerLastLogin(playerID string) error {
	_, err := d.db.Exec(
		"UPDATE players SET last_login = ? WHERE id = ?",
		time.Now(), playerID,
	)
	return err
}

// SaveGame saves a game to the database
func (d *Database) SaveGame(game *game.BlackjackGame) error {
	// Convert game state to JSON
	gameState, err := json.Marshal(game)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`
		INSERT INTO games (id, table_id, created_at, updated_at, status, game_state, min_bet, max_bet)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE
		SET updated_at = $4, status = $5, game_state = $6, min_bet = $7, max_bet = $8
	`,
		game.ID, game.TableID, game.CreatedAt, time.Now(), string(game.Status), gameState, game.MinBet, game.MaxBet)
	return err
}

// GetGame retrieves a game by ID
func (d *Database) GetGame(id string) (*game.BlackjackGame, error) {
	var gameState []byte
	var g game.BlackjackGame

	err := d.db.QueryRow(`
		SELECT game_state FROM games WHERE id = $1
	`, id).Scan(&gameState)

	if err != nil {
		return nil, errors.New("game not found")
	}

	if err := json.Unmarshal(gameState, &g); err != nil {
		return nil, err
	}

	return &g, nil
}

// GetTableGames retrieves all games for a table
func (d *Database) GetTableGames(tableID string) ([]*game.BlackjackGame, error) {
	rows, err := d.db.Query(`
		SELECT game_state FROM games WHERE table_id = $1 ORDER BY created_at DESC
	`, tableID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*game.BlackjackGame
	for rows.Next() {
		var gameState []byte
		if err := rows.Scan(&gameState); err != nil {
			return nil, err
		}

		var g game.BlackjackGame
		if err := json.Unmarshal(gameState, &g); err != nil {
			return nil, err
		}

		games = append(games, &g)
	}

	return games, nil
}

// GetActiveTableGame retrieves the active game for a table
func (d *Database) GetActiveTableGame(tableID string) (*game.BlackjackGame, error) {
	var gameState []byte
	var g game.BlackjackGame

	err := d.db.QueryRow(`
		SELECT game_state FROM games 
		WHERE table_id = $1 AND status != $2 
		ORDER BY created_at DESC LIMIT 1
	`, tableID, string(game.Completed)).Scan(&gameState)

	if err != nil {
		return nil, errors.New("no active game found for table")
	}

	if err := json.Unmarshal(gameState, &g); err != nil {
		return nil, err
	}

	return &g, nil
}

// DeleteGame removes a game from the database
func (d *Database) DeleteGame(id string) error {
	_, err := d.db.Exec("DELETE FROM games WHERE id = $1", id)
	return err
}

// GetAllGames returns all games in the database
func (d *Database) GetAllGames() ([]*game.BlackjackGame, error) {
	rows, err := d.db.Query(`
		SELECT game_state FROM games ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*game.BlackjackGame
	for rows.Next() {
		var gameState []byte
		if err := rows.Scan(&gameState); err != nil {
			return nil, err
		}

		var g game.BlackjackGame
		if err := json.Unmarshal(gameState, &g); err != nil {
			return nil, err
		}

		games = append(games, &g)
	}

	return games, nil
}

// UpdateGameStatus updates a game's status in the database
func (d *Database) UpdateGameStatus(gameID string, status game.GameStatus) error {
	var completedAt interface{}
	if status == game.Completed {
		completedAt = time.Now()
	} else {
		completedAt = nil
	}

	_, err := d.db.Exec(
		"UPDATE games SET status = ?, completed_at = ? WHERE id = ?",
		string(status), completedAt, gameID,
	)
	return err
}

// SaveGameResult saves a game result for a player
func (d *Database) SaveGameResult(gameID, playerID string, bet int, result string, winnings int) error {
	_, err := d.db.Exec(
		"INSERT INTO game_results (game_id, player_id, bet, result, winnings, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		gameID, playerID, bet, result, winnings, time.Now(),
	)
	return err
}

// GetPlayerStats retrieves a player's statistics
func (d *Database) GetPlayerStats(playerID string) (*PlayerStats, error) {
	var stats PlayerStats
	var playerName string

	// Get player name
	err := d.db.QueryRow("SELECT name FROM players WHERE id = ?", playerID).Scan(&playerName)
	if err != nil {
		return nil, err
	}

	// Get total games played
	err = d.db.QueryRow("SELECT COUNT(DISTINCT game_id) FROM game_results WHERE player_id = ?", playerID).Scan(&stats.GamesPlayed)
	if err != nil {
		log.Printf("Error getting games played: %v", err)
	}

	// Get total games won
	err = d.db.QueryRow("SELECT COUNT(DISTINCT game_id) FROM game_results WHERE player_id = ? AND result = 'win'", playerID).Scan(&stats.GamesWon)
	if err != nil {
		log.Printf("Error getting games won: %v", err)
	}

	// Get total bets
	err = d.db.QueryRow("SELECT COALESCE(SUM(bet), 0) FROM game_results WHERE player_id = ?", playerID).Scan(&stats.TotalBets)
	if err != nil {
		log.Printf("Error getting total bets: %v", err)
	}

	// Get total winnings
	err = d.db.QueryRow("SELECT COALESCE(SUM(winnings), 0) FROM game_results WHERE player_id = ?", playerID).Scan(&stats.TotalWinnings)
	if err != nil {
		log.Printf("Error getting total winnings: %v", err)
	}

	// Get last played timestamp
	err = d.db.QueryRow("SELECT MAX(created_at) FROM game_results WHERE player_id = ?", playerID).Scan(&stats.LastPlayed)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error getting last played: %v", err)
	}

	stats.PlayerID = playerID
	stats.PlayerName = playerName

	return &stats, nil
}
