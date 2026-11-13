package bot

import (
	"fmt"
	"sort"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

var learnSuits = [...]rules.SuitClass{rules.SCTrump, rules.SCS, rules.SCH, rules.SCC, rules.SCD}

func learnCandidates(table Table) ([][]rules.Card, error) {
	view := table.View
	seat := view.MySeat
	if seat < 0 || seat >= 4 || view.Phase != game.PhasePlayTrick || view.Trick.TurnSeat != seat || view.Trick.LeaderSeat < 0 || view.Trick.LeaderSeat >= 4 {
		return nil, fmt.Errorf("learned policy requires acting seat in play phase")
	}
	hand := append([]rules.Card(nil), table.Hands[seat]...)
	sort.Slice(hand, func(i, j int) bool { return hand[i].ID < hand[j].ID })
	if len(hand) == 0 {
		return nil, fmt.Errorf("empty acting hand")
	}
	trump := view.Trump.Trump
	lead := view.Trick.Plays[view.Trick.LeaderSeat]
	var moves [][]rules.Card
	// Equivalent deck copies give one strategic action and one softmax entry.
	seen := map[[54]byte]bool{}
	add := func(cards []rules.Card) {
		if len(cards) == 0 {
			return
		}
		var key [54]byte
		for _, c := range cards {
			key[c.ID%54]++
		}
		if seen[key] {
			return
		}
		seen[key] = true
		if lead != nil && len(lead.Cards) > 1 && rules.ValidateHideCheck(hand, cards, lead.Blocks, trump) != nil {
			return
		}
		moves = append(moves, append([]rules.Card(nil), cards...))
	}
	if lead == nil {
		for _, c := range hand {
			add([]rules.Card{c})
		}
		for _, sc := range learnSuits {
			pairs, _ := rules.FindBlocksInHand(hand, trump, sc, rules.BlockPair, 0)
			for _, b := range pairs {
				add(b.Cards)
			}
			for length := 2; length <= len(pairs); length++ {
				bs, _ := rules.FindBlocksInHand(hand, trump, sc, rules.BlockTractor, length)
				for _, b := range bs {
					add(b.Cards)
				}
			}
			add(safeThrowFull(hand, seat, table, sc))
		}
		return moves, nil
	}
	n := len(lead.Cards)
	suited := filterSuit(hand, lead.SuitClass)
	fixed := []rules.Card(nil)
	pool := suited
	if len(suited) <= n {
		fixed = suited
		pool = remaining(hand, suited)
	}
	need := n - len(fixed)
	switch {
	case need == 0:
		add(fixed)
	case n == 1:
		for _, c := range pool {
			add([]rules.Card{c})
		}
	default:
		for _, cards := range structuredFollows(hand, lead, trump) {
			add(cards)
		}
		// Enumerate short combinations; for large moves sample windows at several
		// orderings first so late positions and valuable discards remain reachable.
		if need > 1 {
			for _, byPoints := range []bool{false, true} {
				sort.SliceStable(pool, func(i, j int) bool {
					if byPoints && points(pool[i]) != points(pool[j]) {
						return points(pool[i]) < points(pool[j])
					}
					return pool[i].ID%54 < pool[j].ID%54
				})
				for start := 0; start < len(pool); start++ {
					cards := append([]rules.Card(nil), fixed...)
					for i := 0; i < need; i++ {
						cards = append(cards, pool[(start+i)%len(pool)])
					}
					add(cards)
				}
			}
		}
		attempts := 0
		chosen := append([]rules.Card(nil), fixed...)
		var enumerate func(int, int)
		enumerate = func(start, left int) {
			// Continue until the first legal completion even when the sample budget runs out.
			if attempts >= 96 && len(moves) > 0 {
				return
			}
			if left == 0 {
				attempts++
				add(chosen)
				return
			}
			for i := start; i <= len(pool)-left; i++ {
				chosen = append(chosen, pool[i])
				enumerate(i+1, left-1)
				chosen = chosen[:len(chosen)-1]
				if attempts >= 96 && len(moves) > 0 {
					break
				}
			}
		}
		enumerate(0, need)
	}
	if len(moves) == 0 {
		return nil, fmt.Errorf("no legal learned candidate for %d-card lead", n)
	}
	return moves, nil
}

const learnStateSize = 5*54 + 20
const learnMoveSize = 18

type learnState [learnStateSize]float64
type learnMove [learnMoveSize]float64

func flag(on bool) float64 {
	if on {
		return 1
	}
	return 0
}

func encodeLearnState(table Table) learnState {
	var x learnState
	v := table.View
	seat := v.MySeat
	for offset := 0; offset < 4; offset++ {
		for _, c := range table.Hands[(seat+offset)%4] {
			x[offset*54+c.ID%54] += 0.5
		}
	}
	for _, c := range table.Bottom {
		x[4*54+c.ID%54] += 0.5
	}
	u := x[5*54:]
	u[0] = float64(v.Points) / 120
	u[1] = flag(v.Seats[seat].Team == table.CallerTeam)
	u[2] = flag(table.CanDig)
	u[3] = cardPoints(table.Bottom) / 40
	u[4] = float64(len(table.Hands[seat])) / 25
	u[5] = float64((seat-v.Trick.LeaderSeat+4)%4) / 3
	u[6] = float64(v.Trump.LevelRank.BaseValue()) / 14
	u[7] = float64(v.Teams[v.Seats[seat].Team].LevelRank.BaseValue()) / 14
	u[8] = float64(v.Teams[1-v.Seats[seat].Team].LevelRank.BaseValue()) / 14
	for i, suit := range []rules.Suit{rules.Spade, rules.Heart, rules.Club, rules.Diamond} {
		u[9+i] = flag(v.Trump.HasTrumpSuit && v.Trump.Suit == suit)
	}
	if lead := v.Trick.Plays[v.Trick.LeaderSeat]; lead != nil {
		for i, sc := range learnSuits {
			u[13+i] = flag(lead.SuitClass == sc)
		}
	}
	if b := v.Trick.BiggerSeat; b >= 0 && b < 4 && v.Trick.Plays[b] != nil {
		u[18] = flag(v.Seats[b].Team == v.Seats[seat].Team)
	}
	for _, p := range v.Trick.Plays {
		if p != nil {
			u[19] += cardPoints(p.Cards) / 40
		}
	}
	return x
}

func learnPower(c rules.Card, t rules.Trump) float64 {
	switch {
	case c.Rank == rules.RBJ:
		return 1
	case c.Rank == rules.RSJ:
		return .95
	case c.Rank == t.LevelRank:
		if t.HasTrumpSuit && t.Suit == c.Suit {
			return .9
		}
		return .85
	default:
		return (float64(c.Rank.BaseValue()) + 14*flag(c.SuitClass == rules.SCTrump)) / 35
	}
}

func encodeLearnMove(table Table, cards []rules.Card) learnMove {
	var f learnMove
	v := table.View
	s := v.MySeat
	t := v.Trump.Trump
	var selected, held [54]int
	sc, same := rules.ComputeSuitClassAllSame(cards)
	for _, c := range table.Hands[s] {
		held[c.ID%54]++
		if c.SuitClass == sc {
			f[7]++
		}
	}
	for _, c := range cards {
		selected[c.ID%54]++
		f[1] += points(c) / 20
		f[2] += learnPower(c, t)
		f[3] += flag(c.SuitClass == rules.SCTrump)
		if c.SuitClass == sc {
			f[7]--
		}
	}
	n := float64(len(cards))
	f[0] = n / 6
	f[2] /= n
	f[3] /= n
	for i, count := range selected {
		if count == 2 {
			f[4]++
		}
		if count == 1 && held[i] == 2 {
			f[5]++
		}
	}
	f[4] /= 3
	f[6] = flag(f[7] == 0 && same)
	f[7] /= 12
	lead := v.Trick.Plays[v.Trick.LeaderSeat]
	wins := lead == nil
	ally := false
	if lead != nil {
		b := v.Trick.BiggerSeat
		if b >= 0 && b < 4 && v.Trick.Plays[b] != nil {
			_, eligible, _ := rules.ClassifyPaddingAndComparable(cards, lead.SuitClass)
			if eligible {
				wins, _ = rules.CompareForTrickWin(v.Trick.Plays[b].Blocks, cards, t)
			}
			ally = v.Seats[b].Team == v.Seats[s].Team
		}
	}
	f[8] = flag(wins)
	f[9] = flag(wins && !ally)
	f[10] = flag(wins && ally)
	// Exact single-card responses of remaining players in this trick, including
	// compulsory following. The network learns how valuable that control is.
	winner := v.Trick.BiggerSeat
	if wins {
		winner = s
	}
	if len(cards) == 1 && (lead == nil || len(lead.Cards) == 1) {
		suit := sc
		best := learnPower(cards[0], t)
		if lead != nil {
			suit = lead.SuitClass
			if !wins && winner >= 0 {
				inc := v.Trick.Plays[winner]
				best = learnPower(inc.Cards[0], t)
			}
		}
		for off := 1; off < 4; off++ {
			next := (s + off) % 4
			if v.Trick.Plays[next] != nil {
				continue
			}
			hasSuit := false
			for _, c := range table.Hands[next] {
				if c.SuitClass == suit {
					hasSuit = true
					break
				}
			}
			for _, c := range table.Hands[next] {
				if c.SuitClass != suit && (hasSuit || c.SuitClass != rules.SCTrump) {
					continue
				}
				if power := learnPower(c, t); power > best {
					best = power
					winner = next
				}
			}
		}
	}
	ours := winner >= 0 && v.Seats[winner].Team == v.Seats[s].Team
	f[11] = flag(ours)
	f[12] = f[1] * (2*flag(ours) - 1)
	f[13] = flag(len(table.Hands[s]) == len(cards))
	for i, cls := range learnSuits[1:] {
		f[14+i] = flag(sc == cls)
	}
	return f
}
