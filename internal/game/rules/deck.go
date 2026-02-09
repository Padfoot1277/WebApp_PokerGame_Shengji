package rules

import (
	"math/rand"
	"time"
)

const DoubleDeckSize = 108

func NewDoubleDeck() []Card {
	suits := []Suit{Spade, Heart, Club, Diamond}
	ranks := []Rank{
		RA, RK, RQ, RJ, R10, R9, R8,
		R7, R6, R5, R4, R3, R2,
	}

	deck := make([]Card, 0, DoubleDeckSize)
	id := 0
	// 两副牌
	for d := 0; d < 2; d++ {
		for _, s := range suits {
			for _, r := range ranks {
				deck = append(deck, Card{
					ID:   id,
					Suit: s,
					Rank: r,
				})
				id++
			}
		}
		deck = append(deck, Card{ID: id, Suit: SmallJoker, Rank: RSJ})
		id++
		deck = append(deck, Card{ID: id, Suit: BigJoker, Rank: RBJ})
		id++
	}
	// 总数应为108
	return deck
}

// ShuffleInPlace 最优级别的算法（Fisher–Yates 洗牌）
func ShuffleInPlace(deck []Card) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(deck) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}

// Deal seatsHands[4] each 25, bottom 8
func Deal(deck []Card) (hands [4][]Card, bottom []Card) {
	idx := 0
	for s := 0; s < 4; s++ {
		hands[s] = append(hands[s], deck[idx:idx+25]...)
		idx += 25
	}
	bottom = append(bottom, deck[idx:idx+8]...)
	return
}
