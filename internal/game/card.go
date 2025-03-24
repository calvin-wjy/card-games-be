package game

type Suit string
type Rank string

const (
	Hearts   Suit = "Hearts"
	Diamonds Suit = "Diamonds"
	Clubs    Suit = "Clubs"
	Spades   Suit = "Spades"
)

const (
	Ace   Rank = "Ace"
	Two   Rank = "2"
	Three Rank = "3"
	Four  Rank = "4"
	Five  Rank = "5"
	Six   Rank = "6"
	Seven Rank = "7"
	Eight Rank = "8"
	Nine  Rank = "9"
	Ten   Rank = "10"
	Jack  Rank = "Jack"
	Queen Rank = "Queen"
	King  Rank = "King"
)

type Card struct {
	Suit  Suit `json:"suit"`
	Rank  Rank `json:"rank"`
	Value int  `json:"value,omitempty"`
	Face  bool `json:"face"` // True for face-up cards, false for face-down
}

// GetValue returns the blackjack value of the card
func (c Card) GetValue() int {
	switch c.Rank {
	case Ace:
		return 11 // Ace is 11 by default, but can be 1 in some cases
	case Ten, Jack, Queen, King:
		return 10
	case Two:
		return 2
	case Three:
		return 3
	case Four:
		return 4
	case Five:
		return 5
	case Six:
		return 6
	case Seven:
		return 7
	case Eight:
		return 8
	case Nine:
		return 9
	default:
		return 0
	}
}
