package store

import (
	"github.com/calvinwijaya/card-games-be/internal/db"
	"github.com/calvinwijaya/card-games-be/internal/game"
)

// DatabaseStore is a database implementation of game storage
type DatabaseStore struct {
	db *db.Database
}

// NewDatabaseStore creates a new database store
func NewDatabaseStore(database *db.Database) *DatabaseStore {
	return &DatabaseStore{
		db: database,
	}
}

// SaveGame saves a game to the database
func (s *DatabaseStore) SaveGame(g *game.BlackjackGame) error {
	return s.db.SaveGame(g)
}

// GetGame retrieves a game by ID
func (s *DatabaseStore) GetGame(id string) (*game.BlackjackGame, error) {
	return s.db.GetGame(id)
}

// GetTableGames retrieves all games for a table
func (s *DatabaseStore) GetTableGames(tableID string) ([]*game.BlackjackGame, error) {
	return s.db.GetTableGames(tableID)
}

// GetActiveTableGame retrieves the active game for a table
func (s *DatabaseStore) GetActiveTableGame(tableID string) (*game.BlackjackGame, error) {
	return s.db.GetActiveTableGame(tableID)
}

// DeleteGame removes a game from the database
func (s *DatabaseStore) DeleteGame(id string) error {
	return s.db.DeleteGame(id)
}

// GetAllGames returns all games in the database
func (s *DatabaseStore) GetAllGames() ([]*game.BlackjackGame, error) {
	return s.db.GetAllGames()
}
