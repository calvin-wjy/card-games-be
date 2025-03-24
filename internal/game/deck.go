package game

import (
	"math/rand"
	"time"
)

type Deck struct {
	Cards []Card
}

// NewDeck creates a new standard 52-card deck
func NewDeck() *Deck {
	deck := &Deck{}
	suits := []Suit{Hearts, Diamonds, Clubs, Spades}
	ranks := []Rank{Ace, Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King}

	for _, suit := range suits {
		for _, rank := range ranks {
			card := Card{
				Suit:  suit,
				Rank:  rank,
				Value: 0, // Value is calculated on demand using GetValue()
				Face:  true,
			}
			deck.Cards = append(deck.Cards, card)
		}
	}

	return deck
}

// Shuffle randomizes the order of cards in the deck
func (d *Deck) Shuffle() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Fisher-Yates shuffle algorithm
	for i := len(d.Cards) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	}
}

// DrawCard removes and returns the top card from the deck
func (d *Deck) DrawCard() (Card, bool) {
	if len(d.Cards) == 0 {
		return Card{}, false
	}

	card := d.Cards[0]
	d.Cards = d.Cards[1:]
	return card, true
}

// RemainingCards returns the number of cards left in the deck
func (d *Deck) RemainingCards() int {
	return len(d.Cards)
}
