package bot

import (
	"math/rand"
	"testing"

	"upgrade-lan/internal/game/rules"
)

func TestCachedControlMatchesRemainingCards(t *testing.T) {
	deck := rules.NewDoubleDeck()
	rng := rand.New(rand.NewSource(82))
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	hands, _ := rules.Deal(deck)
	table := Observe(fullFixture(hands), 0)
	q := fullSearch{table: table, afterControl: map[string]float64{}}
	for seat, hand := range table.Hands {
		q.safeBlocks[seat] = controlBlocks(hand, seat, table)
		for _, block := range q.safeBlocks[seat] {
			q.control[seat] += float64(len(block.Cards) * 2)
		}
		for i := 1; i <= len(hand); i++ {
			var expected float64
			for _, b := range controlBlocks(hand[i:], seat, table) {
				expected += float64(2 * len(b.Cards))
			}
			if got := q.controlAfter(seat, hand[:i]); got != expected {
				t.Fatalf("seat %d remove %d cards control %v expected %v", seat, i, got, expected)
			}
		}
	}
}
