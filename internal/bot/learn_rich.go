package bot

import "upgrade-lan/internal/game/rules"

const richFaces = rules.DoubleDeckSize / 2
const richExtraSize = richFaces + 3*len(learnSuits)

type richPair struct {
	face int
	rank int
}

type richContext struct {
	count [len(learnSuits)]int
	suit  [richFaces]int
	pairs [len(learnSuits)][]richPair
}

// Compute suit membership and the rules engine's sorted pair ranks once per
// decision. Removing a move can only break pairs that already exist.
func prepareRich(table Table) richContext {
	var ctx richContext
	hand := table.Hands[table.View.MySeat]
	for i, suit := range learnSuits {
		for _, c := range hand {
			if c.SuitClass == suit {
				ctx.count[i]++
				ctx.suit[c.ID%richFaces] = i
			}
		}
		pairs, _ := rules.FindBlocksInHand(hand, table.View.Trump.Trump, suit, rules.BlockPair, 0)
		for _, pair := range pairs {
			ctx.pairs[i] = append(ctx.pairs[i], richPair{pair.Cards[0].ID % richFaces, pair.RankValue})
		}
	}
	return ctx
}

// Candidate face counts are in half-deck units. Each suit then contributes
// residual card count, pair count, and longest tractor length (in pairs).
func (ctx *richContext) encode(cards []rules.Card, out []float64) {
	clear(out)
	count := ctx.count
	for _, c := range cards {
		face := c.ID % richFaces
		out[face] += 0.5
		count[ctx.suit[face]]--
	}
	for i, pairs := range ctx.pairs {
		pairCount, run, longest, prev := 0, 0, 0, 0
		for _, pair := range pairs {
			if out[pair.face] != 0 {
				continue
			}
			pairCount++
			if run > 0 && prev == pair.rank+1 {
				run++
				longest = max(longest, run)
			} else {
				run = 1
			}
			prev = pair.rank
		}
		j := richFaces + 3*i
		out[j] = float64(count[i]) / 25
		out[j+1] = float64(pairCount) / 6
		out[j+2] = float64(longest) / 6
	}
}
