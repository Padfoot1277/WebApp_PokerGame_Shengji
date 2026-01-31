package rules

type Suit string

const (
	Spade   Suit = "S"
	Heart   Suit = "H"
	Club    Suit = "C"
	Diamond Suit = "D"
)

type Rank string

const (
	RA  Rank = "A"
	RK  Rank = "K"
	RQ  Rank = "Q"
	RJ  Rank = "J"
	R10 Rank = "10"
	R9  Rank = "9"
	R8  Rank = "8"
	R7  Rank = "7"
	R6  Rank = "6"
	R5  Rank = "5"
	R4  Rank = "4"
	R3  Rank = "3"
	R2  Rank = "2"
)

type CardKind string

const (
	KindNormal     CardKind = "normal"
	KindJokerBig   CardKind = "joker_big"
	KindJokerSmall CardKind = "joker_small"
)

type Color string

const (
	Red   Color = "red"
	Black Color = "black"
)

type Card struct {
	ID    int      `json:"id"`
	Kind  CardKind `json:"kind"`
	Suit  Suit     `json:"suit,omitempty"`  // normal 才有
	Rank  Rank     `json:"rank,omitempty"`  // normal 才有
	Color Color    `json:"color,omitempty"` // joker 才有（用于显示）
}
