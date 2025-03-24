package store

import (
	"errors"
	"sync"

	"github.com/calvinwijaya/card-games-be/internal/game"
)

// MemoryStore is an in-memory implementation of game storage
type MemoryStore struct {
	games  map[string]*game.BlackjackGame
	tables map[string][]*game.BlackjackGame
	mu     sync.RWMutex
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		games:  make(map[string]*game.BlackjackGame),
		tables: make(map[string][]*game.BlackjackGame),
	}
}

// SaveGame saves a game to the store
func (s *MemoryStore) SaveGame(g *game.BlackjackGame) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.games[g.ID] = g

	// Add to table games
	tableGames, exists := s.tables[g.TableID]
	if !exists {
		tableGames = []*game.BlackjackGame{}
	}
	s.tables[g.TableID] = append(tableGames, g)

	return nil
}

// GetGame retrieves a game by ID
func (s *MemoryStore) GetGame(id string) (*game.BlackjackGame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, exists := s.games[id]
	if !exists {
		return nil, errors.New("game not found")
	}

	return g, nil
}

// GetTableGames retrieves all games for a table
func (s *MemoryStore) GetTableGames(tableID string) ([]*game.BlackjackGame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	games, exists := s.tables[tableID]
	if !exists {
		return []*game.BlackjackGame{}, nil
	}

	return games, nil
}

// GetActiveTableGame retrieves the active game for a table
func (s *MemoryStore) GetActiveTableGame(tableID string) (*game.BlackjackGame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	games, exists := s.tables[tableID]
	if !exists {
		return nil, errors.New("table not found")
	}

	// Find an active game (one that isn't completed)
	for _, g := range games {
		if g.Status != game.Completed {
			return g, nil
		}
	}

	return nil, errors.New("no active game found for table")
}

// DeleteGame removes a game from the store
func (s *MemoryStore) DeleteGame(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, exists := s.games[id]
	if !exists {
		return errors.New("game not found")
	}

	// Remove from games map
	delete(s.games, id)

	// Remove from table games
	tableGames, exists := s.tables[g.TableID]
	if exists {
		for i, game := range tableGames {
			if game.ID == id {
				// Remove game from slice
				s.tables[g.TableID] = append(tableGames[:i], tableGames[i+1:]...)
				break
			}
		}
	}

	return nil
}

// GetAllGames returns all games in the store
func (s *MemoryStore) GetAllGames() ([]*game.BlackjackGame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	games := make([]*game.BlackjackGame, 0, len(s.games))
	for _, g := range s.games {
		games = append(games, g)
	}

	return games, nil
}
