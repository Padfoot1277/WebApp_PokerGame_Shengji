package rules

// Suit 花色
type Suit string

const (
	Spade   Suit = "S"
	Heart   Suit = "H"
	Club    Suit = "C"
	Diamond Suit = "D"
)

// SuitClass 出牌类别（牌域）
type SuitClass string

const (
	SCTrump   SuitClass = "Trump"
	SCS       SuitClass = "S"
	SCH       SuitClass = "H"
	SCC       SuitClass = "C"
	SCD       SuitClass = "D"
	SCMix     SuitClass = "Mix"
	SCUnknown SuitClass = "Unknown"
)

// Rank 牌号
type Rank string

const (
	RBJ Rank = "JB"
	RSJ Rank = "JS"
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

var RankValues = map[Rank]int{
	RBJ: 16,
	RSJ: 15,
	RA:  14,
	RK:  13,
	RQ:  12,
	RJ:  11,
	R10: 10,
	R9:  9,
	R8:  8,
	R7:  7,
	R6:  6,
	R5:  5,
	R4:  4,
	R3:  3,
	R2:  2,
}

// CardKind 牌等级
type CardKind string

const (
	KindNormal     CardKind = "normal"
	KindJokerBig   CardKind = "joker_big"
	KindJokerSmall CardKind = "joker_small"
)

// Color 牌色
type Color string

const (
	Red   Color = "red"
	Black Color = "black"
)

// Card 牌
type Card struct {
	ID        int       `json:"id"`
	Kind      CardKind  `json:"kind"`
	Suit      Suit      `json:"suit,omitempty"` // normal 才有
	Rank      Rank      `json:"rank"`
	Color     Color     `json:"color,omitempty"`      // joker 才有（用于显示）
	SuitClass SuitClass `json:"suit_class,omitempty"` // 定主后确定牌域
}
