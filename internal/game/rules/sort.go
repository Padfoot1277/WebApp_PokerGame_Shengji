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
				return ai[k] > aj[k]
			}
		}
		return cards[i].ID > cards[j].ID
	})
}

// cardKey returns a tuple-like slice for ordering
func cardKey(c Card, ctx SortCtx) []int {
	// group: 5 big joker, 4 small joker, 3 trump-level, 2 other-level, 1 trump-suit, 0 others

	// 1) Jokers
	if c.Kind == KindJokerBig {
		return []int{5, 0, 0}
	}
	if c.Kind == KindJokerSmall {
		return []int{4, 0, 0}
	}

	// Normal cards
	isLevel := c.Rank == ctx.LevelRank

	// 2) Level cards
	if isLevel {
		// 主级牌：有主且花色=主花色
		if ctx.HasTrumpSuit && c.Suit == ctx.TrumpSuit {
			// 主级牌内部：按花色顺序（其实都同一花色），再按ID稳定
			return []int{3, 0, 0}
		}
		// 副级牌：硬主下所有级牌都在这里
		return []int{2, suitOrder(c.Suit), 0}
	}

	// 3) Trump suit (non-level)
	if ctx.HasTrumpSuit && c.Suit == ctx.TrumpSuit {
		// 主花色普通牌：A..2
		return []int{1, RankValues[c.Rank], 0}
	}

	// 4) Others (non-level, non-trump)
	// 副牌：先按花色顺序，再按点数
	return []int{0, suitOrder(c.Suit), RankValues[c.Rank]}
}

// suit order: ♥, ♠, ♦, ♣
func suitOrder(s Suit) int {
	switch s {
	case Heart:
		return 4
	case Spade:
		return 3
	case Diamond:
		return 2
	case Club:
		return 1
	default:
		return 0
	}
}
