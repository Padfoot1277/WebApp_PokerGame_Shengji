package bot

import (
	"fmt"
	"math"
	"sort"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func points(c rules.Card) float64 {
	if c.Rank == rules.R5 {
		return 5
	}
	if c.Rank == rules.R10 || c.Rank == rules.RK {
		return 10
	}
	return 0
}
func strength(c rules.Card, t rules.Trump) float64 {
	b, _ := rules.FindBlocksInHand([]rules.Card{c}, t, c.SuitClass, rules.BlockSingle, 0)
	if len(b) == 0 {
		return 0
	}
	v := b[0].RankValue
	if v >= 8000 {
		return 20 + float64(v-8000)/200
	}
	if v >= 7000 {
		return 15 + float64(v-7000)
	}
	return float64(v)
}
func choose(s game.ViewState) ([]rules.Card, error) { return chooseWithHistory(s, nil) }
func chooseWithHistory(s game.ViewState, history []PublicTrick) ([]rules.Card, error) {
	if s.MySeat < 0 || s.MySeat > 3 {
		return nil, fmt.Errorf("invalid seat")
	}
	hand := []rules.Card{}
	for _, g := range s.MyHand {
		hand = append(hand, g...)
	}
	if len(hand) == 0 {
		return nil, fmt.Errorf("empty hand")
	}
	t := s.Trump.Trump
	sort.Slice(hand, func(i, j int) bool {
		a, b := strength(hand[i], t)+points(hand[i]), strength(hand[j], t)+points(hand[j])
		if a == b {
			return hand[i].ID < hand[j].ID
		}
		return a < b
	})
	k := knowledge(s, history, hand)
	lead := s.Trick.Plays[s.Trick.LeaderSeat]
	bestScore := math.Inf(-1)
	var best []rules.Card
	// Charge for breaking pairs and tractors, rather than consuming them intact.
	resources := [][]rules.Card{}
	for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
		for _, kind := range []rules.BlockType{rules.BlockPair, rules.BlockTractor} {
			length := 0
			if kind == rules.BlockTractor {
				length = 2
			}
			bs, _ := rules.FindBlocksInHand(hand, t, sc, kind, length)
			for _, b := range bs {
				resources = append(resources, b.Cards)
			}
		}
	}
	evaluate := func(cards []rules.Card) {
		score := 0.0
		ps := 0.0
		power := 0.0
		selected := map[int]bool{}
		for _, c := range cards {
			selected[c.ID] = true
			ps += points(c)
			power += strength(c, t)
		}
		for _, r := range resources {
			n := 0
			for _, c := range r {
				if selected[c.ID] {
					n++
				}
			}
			if n > 0 && n < len(r) {
				score -= 12
			}
		}
		rest := remaining(hand, cards)
		// Retain expensive control cards, especially a last trump before the endgame.
		score -= power * 0.35
		trumps, leftTrumps := 0, 0
		for _, c := range hand {
			if c.SuitClass == rules.SCTrump {
				trumps++
			}
		}
		for _, c := range rest {
			if c.SuitClass == rules.SCTrump {
				leftTrumps++
			}
		}
		if trumps > 0 && leftTrumps == 0 && len(rest) > 2 {
			score -= 16
		}
		// Clearing a short side suit creates future ruff opportunities.
		if len(rest) > 0 && leftTrumps > 0 {
			for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC} {
				before, after := 0, 0
				for _, c := range hand {
					if c.SuitClass == sc {
						before++
					}
				}
				for _, c := range rest {
					if c.SuitClass == sc {
						after++
					}
				}
				if before > 0 && after == 0 {
					score += 5
				}
			}
		}
		if lead == nil {
			blocks, _ := rules.DecomposeThrow(cards, t, cards[0].SuitClass)
			safe := k.survival(s, blocks, cards[0].SuitClass, true)
			score += float64(len(cards)-1)*9 + safe*30 - (1-safe)*ps*2
			// Cash likely winners before opponents can void a side suit.
			score += safe * power * 0.22
			if cards[0].SuitClass == rules.SCTrump {
				score -= 8
				// Draw trump with a long, strong holding; avoid draining a short one.
				if trumps >= len(hand)/2 && trumps >= 5 && safe > 0.65 {
					score += 20
				}
			}
			if len(rest) == 0 {
				score += safe * terminalBonus(s)
			}
		} else {
			big := s.Trick.BiggerSeat
			if big < 0 || big > 3 || s.Trick.Plays[big] == nil {
				return
			}
			incumbent := s.Trick.Plays[big]
			_, comparable, sc := rules.ClassifyPaddingAndComparable(cards, lead.SuitClass)
			blocks, win, err := rules.WinningBlocks(incumbent.Blocks, cards, t)
			win = win && err == nil && comparable
			teamAhead := s.Seats[big].Team == s.Seats[s.MySeat].Team
			teamChance := 0.0
			if win {
				teamChance = k.survival(s, blocks, sc, false)
			} else if teamAhead {
				teamChance = k.survival(s, incumbent.Blocks, incumbent.SuitClass, false)
			}
			pot := 0.0
			for _, p := range s.Trick.Plays {
				if p != nil {
					for _, c := range p.Cards {
						pot += points(c)
					}
				}
			}
			// Feed only when our team is likely to hold; protect exposed partner winners.
			score += (2*teamChance - 1) * (pot + ps) * 1.8
			score += teamChance * 18
			if teamAhead && win {
				score -= 5
			}
			if len(rest) == 0 {
				score += teamChance * terminalBonus(s)
			}
			// Approach scoring thresholds with greater urgency to secure the trick.
			if s.Points < 80 && s.Points+int(pot+ps) >= 80 {
				score += teamChance * 12
			}
		}
		if score > bestScore {
			bestScore = score
			best = append([]rules.Card(nil), cards...)
		}
	}
	if lead == nil {
		for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
			if cards := safeThrow(hand, k, t, sc); len(cards) > 0 {
				evaluate(cards)
			}
		}
		for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
			for _, kind := range []rules.BlockType{rules.BlockSingle, rules.BlockPair, rules.BlockTractor} {
				start, end := 0, 0
				if kind == rules.BlockTractor {
					start, end = 2, len(hand)/2
				}
				for n := start; n <= end; n++ {
					bs, _ := rules.FindBlocksInHand(hand, t, sc, kind, n)
					for _, b := range bs {
						evaluate(b.Cards)
					}
				}
			}
		}
		return best, nil
	}
	n := len(lead.Cards)
	suited, other := []rules.Card{}, []rules.Card{}
	for _, c := range hand {
		if c.SuitClass == lead.SuitClass {
			suited = append(suited, c)
		} else {
			other = append(other, c)
		}
	}
	fixed := []rules.Card{}
	pool := suited
	need := n
	if len(suited) <= n {
		fixed = append(fixed, suited...)
		pool = other
		need = n - len(suited)
	}
	if need > len(pool) {
		return nil, fmt.Errorf("invalid lead count")
	}
	// Keep the best legal move found within a bounded local search.
	deadline := time.Now().Add(2 * time.Second)
	attempts := 0
	chosen := append([]rules.Card(nil), fixed...)
	var search func(int, int)
	search = func(start, left int) {
		if attempts >= 200000 || time.Now().After(deadline) {
			return
		}
		if left == 0 {
			attempts++
			if rules.ValidateHideCheck(hand, chosen, lead.Blocks, t) == nil {
				evaluate(chosen)
			}
			return
		}
		for i := start; i <= len(pool)-left; i++ {
			chosen = append(chosen, pool[i])
			search(i+1, left-1)
			chosen = chosen[:len(chosen)-1]
			if attempts >= 200000 || time.Now().After(deadline) {
				return
			}
		}
	}
	if len(lead.Blocks) == 1 && len(lead.Blocks[0]) == 1 {
		shape := lead.Blocks[0][0]
		for _, sc := range []rules.SuitClass{lead.SuitClass, rules.SCTrump} {
			bs, _ := rules.FindBlocksInHand(hand, t, sc, shape.Type, shape.TractorLen)
			for _, b := range bs {
				if rules.ValidateHideCheck(hand, b.Cards, lead.Blocks, t) == nil {
					evaluate(b.Cards)
				}
			}
		}
	}
	for _, cards := range structuredFollows(hand, lead, t) {
		evaluate(cards)
	}
	search(0, need)
	if best == nil {
		return nil, fmt.Errorf("no legal follow found within search budget")
	}
	return best, nil
}
