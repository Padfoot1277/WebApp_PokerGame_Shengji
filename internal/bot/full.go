package bot

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// Table is an immutable server-only decision snapshot. Extra hands and bottom
// are deliberately excluded from JSON and never added to the client protocol.
type Table struct {
	View       game.ViewState
	Hands      [4][]rules.Card `json:"-"`
	Bottom     []rules.Card    `json:"-"`
	CallerTeam int             `json:"-"`
	CanDig     bool            `json:"-"`
}

func Observe(st game.GameState, seat int) Table {
	table := Table{}
	if seat < 0 || seat > 3 {
		table.View.MySeat = -1
		return table
	}
	// Deep-copy the public trick too: workers must not share mutable room storage.
	table.View = game.MakeView(st, st.Seats[seat].UID)
	table.View.Trick = table.View.Trick.Clone()
	for i, s := range st.Seats {
		table.Hands[i] = classified(s.Hand, st.Trump.Trump)
	}
	table.Bottom = append([]rules.Card(nil), st.Bottom...)
	caller := st.CallerSeat
	if caller < 0 || caller > 3 {
		caller = st.Trump.CallerSeat
	}
	if caller < 0 || caller > 3 {
		caller = st.Trick.LeaderSeat
	}
	if caller < 0 || caller > 3 {
		caller = 0
	}
	table.CallerTeam = st.Seats[caller].Team
	table.CanDig = st.BottomOwnerSeat >= 0 && (st.Trump.HasTrumpSuit || st.Seats[st.BottomOwnerSeat].Team == table.CallerTeam)
	return table
}
func DecideFull(table Table, history []PublicTrick) (Action, error) {
	s := table.View
	if s.MySeat < 0 || s.MySeat > 3 {
		return Action{}, fmt.Errorf("invalid bot seat")
	}
	if (s.Phase == game.PhaseBottom || s.Phase == game.PhaseTrumpFight || s.Phase == game.PhasePlayTrick) && s.SurrenderVotes[(s.MySeat+2)%4] && !s.SurrenderVotes[s.MySeat] {
		return Decide(s, history, true)
	}
	if s.Phase == game.PhaseBottom && s.BottomOwnerSeat == s.MySeat {
		ids, err := bottomFull(table)
		return Action{string(game.EvPutBottom), game.PutBottomPayload{DiscardIDs: ids}}, err
	}
	if s.Phase != game.PhasePlayTrick || s.Trick.TurnSeat != s.MySeat || s.SurrenderVotes[(s.MySeat+2)%4] {
		return Decide(s, history, true)
	}
	cards, err := chooseFull(table)
	if err != nil {
		return Action{}, err
	}
	ids := []int{}
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	return Action{string(game.EvPlayCards), game.PlayCardsPayload{CardIDs: ids}}, nil
}

type fullMove struct {
	cards               []rules.Card
	points, cost, power float64
}
type trickLine struct {
	winner       int
	blocks       [][]rules.Block
	suit         rules.SuitClass
	points, cost float64
	selected     [4][]rules.Card
}
type fullSearch struct {
	table        Table
	fixedBudget  bool
	team         int
	deadline     time.Time
	cache        map[string][]fullMove
	afterControl map[string]float64
	control      [4]float64
	safeBlocks   [4][]rules.Block
	nodes        int
}

func cardPoints(cards []rules.Card) float64 {
	p := 0.0
	for _, c := range cards {
		p += points(c)
	}
	return p
}
func moveKey(cards []rules.Card) string {
	ids := []int{}
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	sort.Ints(ids)
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(strconv.Itoa(id))
		b.WriteByte(',')
	}
	return b.String()
}
func unbeatable(b rules.Block, seat int, table Table) bool {
	t := table.View.Trump.Trump
	for other, h := range table.Hands {
		if other == seat {
			continue
		}
		for _, sc := range []rules.SuitClass{b.SuitClass, rules.SCTrump} {
			bs, _ := rules.FindBlocksInHand(h, t, sc, b.Type, b.TractorLen)
			for _, candidate := range bs {
				cmp, err := rules.CompareTwoBlocks(b, candidate)
				if err == nil && cmp < 0 && rules.ValidateHideCheck(h, candidate.Cards, [][]rules.Block{{b}}, t) == nil {
					return false
				}
			}
		}
	}
	return true
}
func controlBlocks(h []rules.Card, seat int, table Table) []rules.Block {
	var safe []rules.Block
	t := table.View.Trump.Trump
	for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
		for _, kind := range []rules.BlockType{rules.BlockSingle, rules.BlockPair} {
			bs, _ := rules.FindBlocksInHand(h, t, sc, kind, 0)
			for _, b := range bs {
				if unbeatable(b, seat, table) {
					safe = append(safe, b)
				}
			}
		}
	}
	return safe
}
func costOf(cards, hand []rules.Card, seat int, table Table) float64 {
	t := table.View.Trump.Trump
	rest := remaining(hand, cards)
	cost := 0.0
	selected := map[int]bool{}
	for _, c := range cards {
		selected[c.ID] = true
		cost += strength(c, t) * 0.14
	}
	for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
		for _, kind := range []rules.BlockType{rules.BlockPair, rules.BlockTractor} {
			n := 0
			if kind == rules.BlockTractor {
				n = 2
			}
			bs, _ := rules.FindBlocksInHand(hand, t, sc, kind, n)
			for _, b := range bs {
				count := 0
				for _, c := range b.Cards {
					if selected[c.ID] {
						count++
					}
				}
				if count > 0 && count < len(b.Cards) {
					cost += 9
				}
			}
		}
	}
	before, after := len(filterSuit(hand, rules.SCTrump)), len(filterSuit(rest, rules.SCTrump))
	if before > 0 && after == 0 && len(rest) > 2 {
		cost += 8
	}
	if after > 0 {
		for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC} {
			if len(filterSuit(hand, sc)) > 0 && len(filterSuit(rest, sc)) == 0 {
				cost -= 4
			}
		}
	}
	return cost
}
func (q *fullSearch) controlAfter(seat int, cards []rules.Card) float64 {
	key := strconv.Itoa(seat) + "/" + moveKey(cards)
	if value, ok := q.afterControl[key]; ok {
		return value
	}
	// Other players' hands are fixed during this search, so a previously safe
	// single or pair remains safe exactly when all of its cards are retained.
	var played [rules.DoubleDeckSize]bool
	for _, c := range cards {
		played[c.ID] = true
	}
	value := q.control[seat]
	for _, block := range q.safeBlocks[seat] {
		for _, c := range block.Cards {
			if played[c.ID] {
				value -= float64(2 * len(block.Cards))
				break
			}
		}
	}
	q.afterControl[key] = value
	return value
}
func (q *fullSearch) makeMove(cards []rules.Card, seat int) fullMove {
	power := 0.0
	for _, c := range cards {
		power += strength(c, q.table.View.Trump.Trump)
	}
	return fullMove{cards: append([]rules.Card(nil), cards...), points: cardPoints(cards), cost: costOf(cards, q.table.Hands[seat], seat, q.table) + math.Max(0, q.control[seat]-q.controlAfter(seat, cards))*0.5, power: power}
}

// Enumerate legal local candidates and seed whole structures before the bounded
// search. Branch compression later retains cheap wins, safe covers and point dumps.
func (q *fullSearch) follows(seat int, lead *game.PlayedMove) []fullMove {
	key := fmt.Sprintf("%d/%s/%d/", seat, lead.SuitClass, len(lead.Cards))
	for _, g := range lead.Blocks {
		for _, b := range g {
			key += fmt.Sprintf("%s:%d;", b.Type, b.TractorLen)
		}
	}
	if cached, ok := q.cache[key]; ok {
		return cached
	}
	h := q.table.Hands[seat]
	n := len(lead.Cards)
	out := []fullMove{}
	seen := map[string]bool{}
	t := q.table.View.Trump.Trump
	add := func(cards []rules.Card) {
		if len(cards) != n || rules.ValidateHideCheck(h, cards, lead.Blocks, t) != nil {
			return
		}
		k := moveKey(cards)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, q.makeMove(cards, seat))
	}
	for _, cards := range structuredFollows(h, lead, t) {
		add(cards)
	}
	suited := filterSuit(h, lead.SuitClass)
	pool := suited
	fixed := []rules.Card{}
	need := n
	if len(suited) <= n {
		fixed = append(fixed, suited...)
		pool = remaining(h, suited)
		need = n - len(suited)
	}
	chosen := append([]rules.Card(nil), fixed...)
	enumerationDeadline := time.Now().Add(60 * time.Millisecond)
	attempts := 0
	exhausted := func() bool {
		if q.fixedBudget {
			return attempts >= 192 && len(out) > 0 || attempts >= 12000
		}
		return attempts >= 12000 || len(out) > 0 && time.Now().After(enumerationDeadline)
	}
	var search func(int, int)
	search = func(start, left int) {
		if exhausted() {
			return
		}
		if left == 0 {
			attempts++
			add(chosen)
			return
		}
		for i := start; i <= len(pool)-left; i++ {
			chosen = append(chosen, pool[i])
			search(i+1, left-1)
			chosen = chosen[:len(chosen)-1]
			if exhausted() {
				return
			}
		}
	}
	// Sort the pool cheaply in both directions so truncated large searches cover both ends.
	sort.SliceStable(pool, func(i, j int) bool {
		return strength(pool[i], t)+points(pool[i]) < strength(pool[j], t)+points(pool[j])
	})
	search(0, need)
	if exhausted() {
		for i, j := 0, len(pool)-1; i < j; i, j = i+1, j-1 {
			pool[i], pool[j] = pool[j], pool[i]
		}
		attempts = 0
		enumerationDeadline = time.Now().Add(60 * time.Millisecond)
		search(0, need)
	}
	if len(out) == 0 {
		// The old bounded validator search remains a last resort for partially
		// matched complex throws which the structural seeds did not construct.
		fallback := q.table.View
		fallback.MySeat = seat
		fallback.MyHand = [][]rules.Card{h}
		leader := (seat + 3) % 4
		fallback.Trick = game.TrickState{LeaderSeat: leader, BiggerSeat: leader, TurnSeat: seat}
		fallback.Trick.Plays[leader] = lead
		if q.fixedBudget {
			table := q.table
			table.View = fallback
			moves, _ := learnCandidates(table)
			for _, cards := range moves {
				add(cards)
			}
		} else if cards, err := chooseWithHistory(fallback, nil); err == nil {
			add(cards)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].cost < out[j].cost })
	// Keep diversity before caching to bound memory in pathological mixed throws.
	if len(out) > 96 {
		keep := append([]fullMove(nil), out[:32]...)
		byPoints := append([]fullMove(nil), out...)
		sort.SliceStable(byPoints, func(i, j int) bool { return byPoints[i].points > byPoints[j].points })
		keep = append(keep, byPoints[:32]...)
		sort.SliceStable(byPoints, func(i, j int) bool { return byPoints[i].power > byPoints[j].power })
		out = append(keep, byPoints[:32]...)
	}
	q.cache[key] = out
	return out
}
func advance(line trickLine, m fullMove, seat int, lead *game.PlayedMove, t rules.Trump) trickLine {
	_, comparable, sc := rules.ClassifyPaddingAndComparable(m.cards, lead.SuitClass)
	if comparable {
		if blocks, win, err := rules.WinningBlocks(line.blocks, m.cards, t); err == nil && win {
			line.winner = seat
			line.blocks = blocks
			line.suit = sc
		}
	}
	line.points += m.points
	line.selected[seat] = m.cards
	return line
}
func (q *fullSearch) branches(options []fullMove, line trickLine, seat int, lead *game.PlayedMove) []fullMove {
	if len(options) <= 10 {
		return options
	}
	out := []fullMove{}
	buckets := map[string]bool{}
	t := q.table.View.Trump.Trump
	// Distinct score amounts and winning/losing classes keep deliberate ducking and feeding.
	for _, m := range options {
		next := advance(line, m, seat, lead, t)
		sc, _ := rules.ComputeSuitClassAllSame(m.cards)
		bucket := fmt.Sprintf("%t/%.0f/%s", next.winner == seat, m.points, sc)
		if !buckets[bucket] {
			buckets[bucket] = true
			out = append(out, m)
		}
		if len(out) >= 12 {
			break
		}
	}
	// A strongest cover may survive a later opponent even when a cheaper win cannot.
	strongest := -1
	for i, m := range options {
		if advance(line, m, seat, lead, t).winner == seat && (strongest < 0 || m.power > options[strongest].power) {
			strongest = i
		}
	}
	if strongest >= 0 {
		out = append(out, options[strongest])
	}
	// Include the cheapest cover that survives a remaining opponent's whole
	// hand, not just the cheapest current win and an unnecessarily large king.
	for offset := 1; offset < 4; offset++ {
		other := (seat + offset) % 4
		if q.table.View.Seats[other].Team == q.table.View.Seats[seat].Team || q.table.View.Trick.Plays[other] != nil || len(line.selected[other]) > 0 {
			continue
		}
		threats := q.follows(other, lead)
		for _, m := range options {
			next := advance(line, m, seat, lead, t)
			if next.winner != seat {
				continue
			}
			safe := true
			for _, reply := range threats {
				if advance(next, reply, other, lead, t).winner == other {
					safe = false
					break
				}
			}
			if safe {
				out = append(out, m)
				break
			}
		}
	}
	return out
}
func (q *fullSearch) terminal(line trickLine) float64 {
	s := q.table.View
	ours := s.Seats[line.winner].Team == q.team
	attacking := q.team != q.table.CallerTeam
	sign := 1.0
	if !ours {
		sign = -1
	}
	weight := 3.0
	if !attacking && !ours {
		weight = 3.8
	}
	if !attacking && ours {
		weight = 3.3
	}
	value := sign*(line.points*weight+5+math.Min(9, q.controlAfter(line.winner, line.selected[line.winner]))) - line.cost
	last := true
	for seat, h := range q.table.Hands {
		if len(h) > len(line.selected[seat]) {
			last = false
		}
	}
	if last {
		value += sign * 20
		if q.table.CanDig && s.Seats[line.winner].Team != q.table.CallerTeam {
			mul := 1
			for _, g := range line.blocks {
				for _, b := range g {
					if b.Type == rules.BlockPair && mul < 2 {
						mul = 2
					}
					if b.Type == rules.BlockTractor {
						mul = 4
					}
				}
			}
			value += sign * cardPoints(q.table.Bottom) * float64(mul) * weight
		}
	}
	attackerWins := s.Seats[line.winner].Team != q.table.CallerTeam
	if attackerWins {
		for _, threshold := range []int{40, 80, 120, 160, 200} {
			if s.Points < threshold && s.Points+int(line.points) >= threshold {
				value += sign * 9
			}
		}
	}
	return value
}
func (q *fullSearch) forecast(line trickLine, seats []int, lead *game.PlayedMove, depth int, alpha, beta float64) float64 {
	if len(seats) == 0 {
		return q.terminal(line)
	}
	seat := seats[0]
	options := q.branches(q.follows(seat, lead), line, seat, lead)
	if len(options) == 0 {
		// An unresolved follow must never look like a proven safe win.
		return q.terminal(line) - 200
	}
	maximize := q.table.View.Seats[seat].Team == q.team
	best := math.Inf(1)
	if maximize {
		best = math.Inf(-1)
	}
	limited := q.nodes >= 1000 || !q.fixedBudget && time.Now().After(q.deadline)
	// Once the budget is spent, complete the trick greedily instead of treating an
	// unfinished trick as a guaranteed win. This still sees each remaining hand.
	if limited {
		chosen := options[0]
		value := math.Inf(1)
		if maximize {
			value = math.Inf(-1)
		}
		for _, m := range options {
			next := advance(line, m, seat, lead, q.table.View.Trump.Trump)
			cost := m.cost
			if !maximize {
				cost = -cost
			}
			next.cost += cost
			v := q.terminal(next)
			if maximize && v > value || !maximize && v < value {
				value = v
				chosen = m
			}
		}
		next := advance(line, chosen, seat, lead, q.table.View.Trump.Trump)
		cost := chosen.cost
		if !maximize {
			cost = -cost
		}
		next.cost += cost
		return q.forecast(next, seats[1:], lead, depth+1, alpha, beta)
	}
	for _, m := range options {
		q.nodes++
		next := advance(line, m, seat, lead, q.table.View.Trump.Trump)
		cost := m.cost
		if !maximize {
			cost = -cost
		}
		next.cost += cost
		value := q.forecast(next, seats[1:], lead, depth+1, alpha, beta)
		if maximize {
			best = math.Max(best, value)
			alpha = math.Max(alpha, best)
		} else {
			best = math.Min(best, value)
			beta = math.Min(beta, best)
		}
		if beta <= alpha {
			break
		}
	}
	return best
}
func chooseFull(table Table) ([]rules.Card, error) {
	return scoreFull(table, nil)
}

// With a visitor the tactical scores are also available for the hybrid policy.
// Fixed work budgets make its shortlist independent of host load.
func scoreFull(table Table, visit func([]rules.Card, float64)) ([]rules.Card, error) {
	return scoreFullBudget(table, visit, visit != nil)
}
func scoreFullBudget(table Table, visit func([]rules.Card, float64), fixedBudget bool) ([]rules.Card, error) {
	s := table.View
	seat := s.MySeat
	t := s.Trump.Trump
	h := table.Hands[seat]
	if len(h) == 0 {
		return nil, fmt.Errorf("empty full-state hand")
	}
	q := fullSearch{table: table, team: s.Seats[seat].Team, fixedBudget: fixedBudget, deadline: time.Now().Add(1500 * time.Millisecond), cache: map[string][]fullMove{}, afterControl: map[string]float64{}}
	for i, hand := range table.Hands {
		q.safeBlocks[i] = controlBlocks(hand, i, table)
		for _, block := range q.safeBlocks[i] {
			q.control[i] += float64(2 * len(block.Cards))
		}
	}
	lead := s.Trick.Plays[s.Trick.LeaderSeat]
	roots := []fullMove{}
	if lead == nil {
		for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
			if cards := safeThrowFull(h, seat, table, sc); len(cards) > 0 {
				roots = append(roots, q.makeMove(cards, seat))
			}
			for _, kind := range []rules.BlockType{rules.BlockSingle, rules.BlockPair, rules.BlockTractor} {
				start, end := 0, 0
				if kind == rules.BlockTractor {
					start, end = 2, len(h)/2
				}
				for n := start; n <= end; n++ {
					bs, _ := rules.FindBlocksInHand(h, t, sc, kind, n)
					for _, b := range bs {
						roots = append(roots, q.makeMove(b.Cards, seat))
					}
				}
			}
		}
	} else {
		roots = q.follows(seat, lead)
	}
	best := math.Inf(-1)
	var chosen []rules.Card
	pending := make([]int, 0, 3)
	for offset := 1; offset < 4; offset++ {
		next := (seat + offset) % 4
		if s.Trick.Plays[next] == nil {
			pending = append(pending, next)
		}
	}
	for _, m := range roots {
		currentLead := lead
		line := trickLine{cost: m.cost}
		line.selected[seat] = m.cards
		if lead == nil {
			sc, _ := rules.ComputeSuitClassAllSame(m.cards)
			blocks, _ := rules.DecomposeThrow(m.cards, t, sc)
			currentLead = &game.PlayedMove{SuitClass: sc, Move: game.Move{Cards: m.cards, Blocks: blocks}}
			line.winner = seat
			line.blocks = blocks
			line.suit = sc
			line.points = m.points
		} else {
			line.winner = s.Trick.BiggerSeat
			if line.winner < 0 || line.winner > 3 || s.Trick.Plays[line.winner] == nil {
				return nil, fmt.Errorf("invalid current winner")
			}
			incumbent := s.Trick.Plays[line.winner]
			line.blocks = incumbent.Blocks
			line.suit = incumbent.SuitClass
			for i, p := range s.Trick.Plays {
				if p != nil {
					line.points += cardPoints(p.Cards)
					line.selected[i] = p.Cards
				}
			}
			line = advance(line, m, seat, lead, t)
		}
		q.nodes = 0
		value := q.forecast(line, pending, currentLead, 0, math.Inf(-1), math.Inf(1))
		// Later tricks can exploit long structures, but never outweigh handing over a loaded trick.
		if lead == nil {
			value += float64(len(m.cards)-1) * 1.5
		}
		if value > best {
			best = value
			chosen = m.cards
		}
		if visit != nil {
			visit(m.cards, value)
		}
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("no legal full-state move")
	}
	return chosen, nil
}

// Throw blocking is checked per actual hand, including our partner. Combining
// opponents' hands would invent pairs or tractors that nobody really holds.
func safeThrowFull(hand []rules.Card, seat int, table Table, sc rules.SuitClass) []rules.Card {
	t := table.View.Trump.Trump
	groups, _ := rules.DecomposeThrow(filterSuit(hand, sc), t, sc)
	selected := []rules.Card{}
	count := 0
	for _, g := range groups {
		for _, b := range g {
			safe := true
			for other, h := range table.Hands {
				if other == seat {
					continue
				}
				bs, _ := rules.FindBlocksInHand(h, t, sc, b.Type, b.TractorLen)
				for _, candidate := range bs {
					if cmp, err := rules.CompareTwoBlocks(b, candidate); err == nil && cmp < 0 {
						safe = false
						break
					}
				}
				if !safe {
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
