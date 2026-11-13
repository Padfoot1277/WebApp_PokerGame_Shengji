package bot

import (
	"math/rand"
	"testing"

	"upgrade-lan/internal/game/rules"
)

func TestRichRetainedTractor(t *testing.T) {
	var hand []rules.Card
	for _, r := range []rules.Rank{rules.R4, rules.R5, rules.R6} {
		for copy := range 2 {
			hand = append(hand, realCard(rules.Club, r, copy))
		}
	}
	table := Observe(fullFixture([4][]rules.Card{hand}), 0)
	ctx := prepareRich(table)
	var f [richExtraSize]float64
	// The club slot follows trump, spades and hearts.
	club := richFaces + 3*3
	ctx.encode(hand[:2], f[:])
	if f[hand[0].ID%richFaces] != 1 || f[club+1] != 2./6 || f[club+2] != 2./6 {
		t.Fatal("leading 44 should retain 55-66", f)
	}
	ctx.encode(hand[2:4], f[:])
	if f[hand[0].ID%richFaces] != 0 || f[club+1] != 2./6 || f[club+2] != 0 {
		t.Fatal("leading 55 should break the tractor", f)
	}
}

func TestRichMatchesRuleShapes(t *testing.T) {
	rng := rand.New(rand.NewSource(550))
	for trial := range 32 {
		deck := rules.NewDoubleDeck()
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		hands, _ := rules.Deal(deck)
		st := fullFixture(hands)
		st.Trump.LevelRank = deck[0].Rank
		if st.Trump.LevelRank.BaseValue() > 14 {
			st.Trump.LevelRank = rules.R2
		}
		st.Trump.HasTrumpSuit = trial%2 == 0
		table := Observe(st, 0)
		ctx := prepareRich(table)
		hand := table.Hands[0]
		for n := 0; n <= len(hand); n += 3 {
			var f [richExtraSize]float64
			ctx.encode(hand[:n], f[:])
			for i, suit := range learnSuits {
				left := hand[n:]
				pairs, _ := rules.FindBlocksInHand(left, st.Trump.Trump, suit, rules.BlockPair, 0)
				longest := 0
				for l := 2; l <= len(pairs); l++ {
					if b, _ := rules.FindBlocksInHand(left, st.Trump.Trump, suit, rules.BlockTractor, l); len(b) > 0 {
						longest = l
					}
				}
				j := richFaces + 3*i
				if f[j] != float64(len(filterSuit(left, suit)))/25 || f[j+1] != float64(len(pairs))/6 || f[j+2] != float64(longest)/6 {
					t.Fatalf("trial %d removing %d cards from suit %s: got %v", trial, n, suit, f[j:j+3])
				}
			}
		}
	}
}
