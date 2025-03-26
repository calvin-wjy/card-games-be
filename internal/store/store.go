package store

import "github.com/calvinwijaya/card-games-be/internal/game"

// Store defines the interface for game storage
type Store interface {
	// SaveGame saves a game to the store
	SaveGame(g *game.BlackjackGame) error

	// GetGame retrieves a game by ID
	GetGame(id string) (*game.BlackjackGame, error)

	// GetTableGames retrieves all games for a table
	GetTableGames(tableID string) ([]*game.BlackjackGame, error)

	// GetActiveTableGame retrieves the active game for a table
	GetActiveTableGame(tableID string) (*game.BlackjackGame, error)

	// DeleteGame removes a game from the store
	DeleteGame(id string) error

	// GetAllGames returns all games in the store
	GetAllGames() ([]*game.BlackjackGame, error)
}
