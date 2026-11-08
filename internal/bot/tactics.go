package bot

import (
	"math"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// Only public plays are retained; no opponent hand or server-private state is used.
type PublicTrick struct {
	Leader int                 `json:"leader"`
	Plays  [4]*game.PlayedMove `json:"plays"`
}
type Input struct {
	game.ViewState
	History []PublicTrick `json:"botHistory"`
}
type tableKnowledge struct {
	unknown []rules.Card
	void    [4]map[rules.SuitClass]bool
}

func knowledge(s game.ViewState, history []PublicTrick, hand []rules.Card) tableKnowledge {
	k := tableKnowledge{}
	seen := map[int]bool{}
	for i := range k.void {
		k.void[i] = map[rules.SuitClass]bool{}
	}
	for _, c := range hand {
		seen[c.ID] = true
	}
	history = append(append([]PublicTrick(nil), history...), PublicTrick{s.Trick.LastLeaderSeat, s.Trick.LastPlays}, PublicTrick{s.Trick.LeaderSeat, s.Trick.Plays})
	for _, tr := range history {
		if tr.Leader < 0 || tr.Leader > 3 {
			continue
		}
		lead := tr.Plays[tr.Leader]
		for seat, p := range tr.Plays {
			if p == nil {
				continue
			}
			for _, c := range p.Cards {
				seen[c.ID] = true
			}
			if lead != nil && seat != tr.Leader {
				count := 0
				for _, c := range p.Cards {
					if rules.ComputeSuitClass(c, s.Trump.Trump) == lead.SuitClass {
						count++
					}
				}
				if count < len(lead.Cards) {
					k.void[seat][lead.SuitClass] = true
				}
			}
		}
	}
	for _, c := range classified(rules.NewDoubleDeck(), s.Trump.Trump) {
		if !seen[c.ID] {
			k.unknown = append(k.unknown, c)
		}
	}
	return k
}
func holdingProbability(total, count, need int) float64 {
	if count < need || total < need {
		return 0
	}
	p := 1.0
	for i := 0; i < need; i++ {
		p *= float64(count-i) / float64(total-i)
	}
	return p
}

// A conservative union estimate of an unseen opponent holding a beating block.
// This is a heuristic, not a calibrated win probability or perfect-information search.
func (k tableKnowledge) threat(s game.ViewState, seat int, blocks [][]rules.Block, sc rules.SuitClass) float64 {
	if len(blocks) == 0 {
		return 1
	}
	if len(blocks) != 1 || len(blocks[0]) != 1 {
		// To beat a throw, the opponent must beat every component. The minimum
		// component risk is an upper bound without assuming independent holdings.
		risk := 1.0
		for _, g := range blocks {
			for _, b := range g {
				risk = math.Min(risk, k.threat(s, seat, [][]rules.Block{{b}}, sc))
			}
		}
		return risk
	}
	target := blocks[0][0]
	risk := 0.0
	domains := []rules.SuitClass{sc}
	if sc != rules.SCTrump {
		domains = append(domains, rules.SCTrump)
	}
	for _, candidateSC := range domains {
		// Same-domain cards cannot be held by a player already known to be void.
		if candidateSC == sc && k.void[seat][sc] {
			continue
		}
		bs, _ := rules.FindBlocksInHand(k.unknown, s.Trump.Trump, candidateSC, target.Type, target.TractorLen)
		for _, b := range bs {
			cmp, err := rules.CompareTwoBlocks(target, b)
			if err != nil || cmp >= 0 {
				continue
			}
			p := holdingProbability(len(k.unknown), s.Seats[seat].HandCount, len(b.Cards))
			if candidateSC != sc {
				if !k.void[seat][sc] { // Unknown void: discount ruff risk using unseen suit occupancy.
					suitCount := 0
					for _, c := range k.unknown {
						if c.SuitClass == sc {
							suitCount++
						}
					}
					p *= math.Pow(1-float64(suitCount)/math.Max(1, float64(len(k.unknown))), float64(s.Seats[seat].HandCount))
				}
			}
			risk += p
		}
	}
	return math.Min(0.95, risk)
}
func (k tableKnowledge) survival(s game.ViewState, blocks [][]rules.Block, sc rules.SuitClass, leading bool) float64 {
	p := 1.0
	for seat := 0; seat < 4; seat++ {
		if seat == s.MySeat || s.Seats[seat].Team == s.Seats[s.MySeat].Team {
			continue
		}
		if !leading && s.Trick.Plays[seat] != nil {
			continue
		}
		p *= 1 - k.threat(s, seat, blocks, sc)
	}
	return p
}
func remaining(hand, selected []rules.Card) []rules.Card {
	used := map[int]bool{}
	for _, c := range selected {
		used[c.ID] = true
	}
	out := []rules.Card{}
	for _, c := range hand {
		if !used[c.ID] {
			out = append(out, c)
		}
	}
	return out
}
func terminalBonus(s game.ViewState) float64 {
	// Both teams care about the last trick; an eligible bottom magnifies its value.
	bonus := 35.0
	if s.Trump.CallerSeat >= 0 && s.Trump.HasTrumpSuit {
		bonus += 30
	}
	return bonus
}

// Offer a throw only when no unseen same-domain block can stop any component.
// Unknown cards include the hidden bottom, making this deliberately conservative.
func safeThrow(hand []rules.Card, k tableKnowledge, t rules.Trump, sc rules.SuitClass) []rules.Card {
	groups, _ := rules.DecomposeThrow(filterSuit(hand, sc), t, sc)
	selected := []rules.Card{}
	count := 0
	for _, g := range groups {
		for _, b := range g {
			candidates, _ := rules.FindBlocksInHand(k.unknown, t, sc, b.Type, b.TractorLen)
			safe := true
			for _, other := range candidates {
				cmp, err := rules.CompareTwoBlocks(b, other)
				if err == nil && cmp < 0 {
					safe = false
					break
				}
			}
			if safe {
				selected = append(selected, b.Cards...)
				count++
			}
		}
	}
	if count < 2 {
		return nil
	}
	return selected
}
func filterSuit(hand []rules.Card, sc rules.SuitClass) []rules.Card {
	out := []rules.Card{}
	for _, c := range hand {
		if c.SuitClass == sc {
			out = append(out, c)
		}
	}
	return out
}

// Seed structurally complete follows before the bounded combination search.
// Both high-first and low-first assignments avoid a search-order bias against
// large winning combinations; the server validator still decides legality.
func structuredFollows(hand []rules.Card, lead *game.PlayedMove, t rules.Trump) [][]rules.Card {
	out := [][]rules.Card{}
	n := len(lead.Cards)
	for _, sc := range []rules.SuitClass{lead.SuitClass, rules.SCTrump} {
		for _, high := range []bool{false, true} {
			pool := filterSuit(hand, sc)
			chosen := []rules.Card{}
			for _, g := range lead.Blocks {
				for _, target := range g {
					bs, _ := rules.FindBlocksInHand(pool, t, sc, target.Type, target.TractorLen)
					if len(bs) == 0 {
						continue
					}
					idx := len(bs) - 1
					if high {
						idx = 0
					}
					b := bs[idx]
					chosen = append(chosen, b.Cards...)
					pool = remaining(pool, b.Cards)
				}
			}
			// When exact structure is unavailable, preserve as many pairs as possible.
			if len(chosen) < n {
				bs, _ := rules.FindBlocksInHand(pool, t, sc, rules.BlockPair, 0)
				for _, b := range bs {
					if len(chosen)+2 <= n {
						chosen = append(chosen, b.Cards...)
						pool = remaining(pool, b.Cards)
					}
				}
			}
			for _, c := range pool {
				if len(chosen) < n {
					chosen = append(chosen, c)
				}
			}
			if len(chosen) == n && rules.ValidateHideCheck(hand, chosen, lead.Blocks, t) == nil {
				out = append(out, chosen)
			}
		}
	}
	return out
}
