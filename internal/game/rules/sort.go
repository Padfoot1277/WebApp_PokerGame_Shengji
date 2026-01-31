package rules

import (
	"sort"
)

type SortCtx struct {
	LevelRank    Rank
	HasTrumpSuit bool
	TrumpSuit    Suit // 仅 HasTrumpSuit=true 时有效
}

// SortHand sorts in-place for UI display.
func SortHand(cards []Card, ctx SortCtx) {
	sort.SliceStable(cards, func(i, j int) bool {
		ai := cardKey(cards[i], ctx)
		aj := cardKey(cards[j], ctx)

		// lexicographic compare
		for k := 0; k < len(ai) && k < len(aj); k++ {
			if ai[k] != aj[k] {
				// key 越小表示越靠前（越大）
				return ai[k] < aj[k]
			}
		}
		return cards[i].ID < cards[j].ID
	})
}

// cardKey returns a tuple-like slice for ordering: smaller is "bigger / earlier".
func cardKey(c Card, ctx SortCtx) []int {
	// group: 0 big joker, 1 small joker, 2 trump-level, 3 other-level, 4 trump-suit, 5 others
	// within each group, define further ordering keys.

	// 1) Jokers
	if c.Kind == KindJokerBig {
		return []int{0, 0, 0}
	}
	if c.Kind == KindJokerSmall {
		return []int{1, 0, 0}
	}

	// Normal cards
	isLevel := c.Rank == ctx.LevelRank

	// 2) Level cards
	if isLevel {
		// 主级牌：有主且花色=主花色
		if ctx.HasTrumpSuit && c.Suit == ctx.TrumpSuit {
			// 主级牌内部：按花色顺序（其实都同一花色），再按ID稳定
			return []int{2, 0, 0}
		}
		// 副级牌：硬主下所有级牌都在这里
		return []int{3, suitOrder(c.Suit), 0}
	}

	// 3) Trump suit (non-level)
	if ctx.HasTrumpSuit && c.Suit == ctx.TrumpSuit {
		// 主花色普通牌：A..2
		return []int{4, rankOrder(c.Rank), 0}
	}

	// 4) Others (non-level, non-trump)
	// 副牌：先按花色顺序，再按点数
	return []int{5, suitOrder(c.Suit), rankOrder(c.Rank)}
}

// suit order: ♥, ♠, ♦, ♣
func suitOrder(s Suit) int {
	switch s {
	case Heart:
		return 0
	case Spade:
		return 1
	case Diamond:
		return 2
	case Club:
		return 3
	default:
		return 9
	}
}

// rank order: A (0) ... 2 (12)
func rankOrder(r Rank) int {
	switch r {
	case RA:
		return 0
	case RK:
		return 1
	case RQ:
		return 2
	case RJ:
		return 3
	case R10:
		return 4
	case R9:
		return 5
	case R8:
		return 6
	case R7:
		return 7
	case R6:
		return 8
	case R5:
		return 9
	case R4:
		return 10
	case R3:
		return 11
	case R2:
		return 12
	default:
		return 99
	}
}
