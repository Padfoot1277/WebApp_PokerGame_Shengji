package bot

import (
	"fmt"
	"math"
	"sort"
	"upgrade-lan/internal/game/rules"
)

// Evaluate whole discard sets, then improve them by swaps. Explicit short-suit
// seeds avoid the old prefix beam's bias against clearing a complete suit.
func bottomFull(table Table) ([]int, error) {
	seat := table.View.MySeat
	h := table.Hands[seat]
	t := table.View.Trump.Trump
	if len(h) != 33 {
		return nil, fmt.Errorf("bottom requires 33 cards, got %d", len(h))
	}
	original := map[rules.SuitClass]int{}
	cardValue := map[int]float64{}
	protected := map[int]bool{}
	resources := []rules.Block{}
	for _, c := range h {
		original[c.SuitClass]++
		cardValue[c.ID] = strength(c, t) * 0.4
		if c.SuitClass == rules.SCTrump {
			cardValue[c.ID] += 6
		}
		if c.Rank == rules.RA {
			cardValue[c.ID] += 7
			protected[c.ID] = true
		}
	}
	for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
		for _, kind := range []rules.BlockType{rules.BlockSingle, rules.BlockPair, rules.BlockTractor} {
			n := 0
			if kind == rules.BlockTractor {
				n = 2
			}
			bs, _ := rules.FindBlocksInHand(h, t, sc, kind, n)
			for _, b := range bs {
				safe := unbeatable(b, seat, table)
				if kind != rules.BlockSingle {
					resources = append(resources, b)
				}
				if safe {
					for _, c := range b.Cards {
						cardValue[c.ID] += 5
						if b.Type == rules.BlockSingle && c.Rank == rules.RA {
							cardValue[c.ID] += 10
						}
						protected[c.ID] = true
					}
				}
			}
		}
	}
	score := func(ids []int) float64 {
		discarded := map[int]bool{}
		for _, id := range ids {
			discarded[id] = true
		}
		value := 0.0
		counts := map[rules.SuitClass]int{}
		weak := map[rules.SuitClass]int{}
		for _, c := range h {
			if discarded[c.ID] {
				penalty := 1.3
				if !table.CanDig {
					penalty = 0.9
				}
				value -= points(c) * penalty
				continue
			}
			value += cardValue[c.ID]
			counts[c.SuitClass]++
			if !protected[c.ID] {
				weak[c.SuitClass]++
			}
		}
		for _, b := range resources {
			kept := 0
			for _, c := range b.Cards {
				if !discarded[c.ID] {
					kept++
				}
			}
			if kept == len(b.Cards) {
				if b.Type == rules.BlockPair {
					value += 10
				} else {
					value += 14
				}
			} else if kept > 0 {
				value -= 8
			}
		}
		if counts[rules.SCTrump] > 0 {
			for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC} {
				if original[sc] == 0 {
					continue
				}
				if counts[sc] == 0 {
					value += 14
				} else if original[sc] <= 6 {
					value -= float64(weak[sc]) * 3
				}
			}
		}
		return value
	}
	order := append([]rules.Card(nil), h...)
	sort.SliceStable(order, func(i, j int) bool {
		a, b := cardValue[order[i].ID]+points(order[i])*1.3, cardValue[order[j].ID]+points(order[j])*1.3
		if a == b {
			return order[i].ID < order[j].ID
		}
		return a < b
	})
	complete := func(seed []int) []int {
		ids := append([]int(nil), seed...)
		seen := map[int]bool{}
		for _, id := range ids {
			seen[id] = true
		}
		for _, c := range order {
			if len(ids) >= 8 {
				break
			}
			if !seen[c.ID] {
				ids = append(ids, c.ID)
				seen[c.ID] = true
			}
		}
		return ids
	}
	seeds := [][]int{complete(nil)}
	if legacy, err := bottom(h, t); err == nil {
		seeds = append(seeds, legacy)
	}
	for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC} {
		for _, keepControls := range []bool{false, true} {
			seed := []int{}
			for _, c := range h {
				if c.SuitClass == sc && (!keepControls || !protected[c.ID]) {
					seed = append(seed, c.ID)
				}
			}
			if len(seed) > 0 && len(seed) <= 8 {
				seeds = append(seeds, complete(seed))
			}
		}
	}
	best := math.Inf(-1)
	var chosen []int
	for _, seed := range seeds {
		ids := append([]int(nil), seed...)
		value := score(ids)
		for round := 0; round < 6; round++ {
			selected := map[int]bool{}
			for _, id := range ids {
				selected[id] = true
			}
			improved := false
			var next []int
			nextScore := value
			for i := range ids {
				for _, c := range h {
					if selected[c.ID] {
						continue
					}
					candidate := append([]int(nil), ids...)
					candidate[i] = c.ID
					v := score(candidate)
					if v > nextScore+0.0001 {
						nextScore = v
						next = candidate
						improved = true
					}
				}
			}
			if !improved {
				break
			}
			ids = next
			value = nextScore
		}
		if value > best {
			best = value
			chosen = ids
		}
	}
	return chosen, nil
}
