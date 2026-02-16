package rules

import "sort"

type CardKey struct{ a, b, c int }

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

// getCardKey returns a tuple-like slice for ordering
func getCardKey(c Card, ctx Trump) CardKey {
	// group: 5 big joker, 4 small joker, 3 trump-level, 2 other-level, 1 trump-suit, 0 others

	// 1) Jokers
	if IsBigJoker(c) {
		return CardKey{a: 5}
	}
	if IsSmallJoker(c) {
		return CardKey{a: 4}
	}

	// 2) Level cards
	isLevel := c.Rank == ctx.LevelRank
	if isLevel {
		if ctx.HasTrumpSuit && c.Suit == ctx.Suit {
			return CardKey{a: 3}
		}
		return CardKey{2, suitOrder(c.Suit), 0}
	}

	// 3) Trump suit (non-level)
	if ctx.HasTrumpSuit && c.Suit == ctx.Suit {
		return CardKey{1, c.Rank.BaseValue(), 0}
	}

	// 4) Others (non-level, non-trump) 副牌：先按花色顺序，再按点数
	return CardKey{0, suitOrder(c.Suit), c.Rank.BaseValue()}
}

// SortHand sorts in-place for UI display.
func SortHand(cards []Card, ctx Trump) {
	sort.SliceStable(cards, func(i, j int) bool {
		cki := getCardKey(cards[i], ctx)
		ckj := getCardKey(cards[j], ctx)

		if cki.a != ckj.a {
			return cki.a > ckj.a
		}
		if cki.b != ckj.b {
			return cki.b > ckj.b
		}
		if cki.c != ckj.c {
			return cki.c > ckj.c
		}
		return cards[i].ID > cards[j].ID
	})
}
