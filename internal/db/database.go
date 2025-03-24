package db

import (
	"database/sql"
	"log"
	"time"

	"github.com/calvinwijaya/card-games-be/internal/game"
	_ "github.com/mattn/go-sqlite3"
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
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
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
		return err
	}

	// Games table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			table_id TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP,
			status TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Game results table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS game_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
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
		return err
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
		"INSERT INTO players (id, name, balance, created_at, last_login) VALUES (?, ?, ?, ?, ?)",
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
	_, err := d.db.Exec(
		"INSERT INTO games (id, table_id, created_at, status) VALUES (?, ?, ?, ?)",
		game.ID, game.TableID, game.CreatedAt, string(game.Status),
	)
	return err
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
