package rules

// Suit 花色
type Suit string

const (
	Spade      Suit = "S"
	Heart      Suit = "H"
	Club       Suit = "C"
	Diamond    Suit = "D"
	SmallJoker Suit = "SJ"
	BigJoker   Suit = "BJ"
)

// SuitClass 出牌类别（牌域）
type SuitClass string

const (
	SCTrump   SuitClass = "主牌"
	SCS       SuitClass = "黑桃"
	SCH       SuitClass = "红桃"
	SCC       SuitClass = "梅花"
	SCD       SuitClass = "方块"
	SCMix     SuitClass = "杂牌"
	SCUnknown SuitClass = "未知"
)

// Rank 牌号
type Rank string

const (
	RBJ Rank = "BJ"
	RSJ Rank = "SJ"
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

	RPending Rank = "Pending"
)

// BaseValue 用于排序显示/基础分组
func (r Rank) BaseValue() int {
	switch r {
	case RBJ:
		return 16
	case RSJ:
		return 15
	case RA:
		return 14
	case RK:
		return 13
	case RQ:
		return 12
	case RJ:
		return 11
	case R10:
		return 10
	case R9:
		return 9
	case R8:
		return 8
	case R7:
		return 7
	case R6:
		return 6
	case R5:
		return 5
	case R4:
		return 4
	case R3:
		return 3
	case R2:
		return 2
	default:
		return 0
	}
}

// Card 牌
type Card struct {
	ID        int       `json:"id"`
	Suit      Suit      `json:"suit,omitempty"` // 原花色
	Rank      Rank      `json:"rank"`
	SuitClass SuitClass `json:"suit_class,omitempty"` // 牌域（定主后确定，但Card存动态属性是否合理？这里仅用于快速判断主副牌
}
