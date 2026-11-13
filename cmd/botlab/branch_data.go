package main

import (
	"flag"
	"fmt"
	"math/rand"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
)

type branchDeal struct {
	Table                           bot.Table
	State                           game.GameState
	Choices                         []endgame.Choice // Fixed-policy rollout values, not minimax values.
	PlayIndex, Candidates, Eligible int
}

// branchValue holds the other three players AND the root player's later
// decisions fixed to the same policy. Score every candidate from the root team.
func branchValue(st game.GameState, cards []int, visitor *bot.LearnModel) (int, error) {
	team := st.Seats[st.Trick.TurnSeat].Team
	next, err := arena.Step(st, cards)
	if err != nil {
		return 0, err
	}
	final, err := arena.Play(next, (policy{visitor, "raw"}).play, nil)
	if err != nil {
		return 0, err
	}
	return arena.Reward(final, team), nil
}

// Stratify by policy rank: keep the top two and include one alternative from
// each half of the tail. A single candidate has exactly one entry.
func branchRanks(n int, rng *rand.Rand) []int {
	if n <= 4 {
		ranks := make([]int, n)
		for i := range ranks {
			ranks[i] = i
		}
		return ranks
	}
	split := 2 + (n-2)/2
	return []int{0, 1, 2 + rng.Intn(split-2), split + rng.Intn(n-split)}
}

func collectBranchDeal(visitor *bot.LearnModel, seed int64, lateTop3 bool) (branchDeal, error) {
	var d branchDeal
	st, err := arena.NewRound(seed)
	if err != nil {
		return d, err
	}
	p := policy{visitor, "raw"}
	minHand, maxHand := 6, 10
	if lateTop3 {
		minHand, maxHand = 4, 6
	}
	// Reservoir sample by turn, before computing any candidate count or label.
	// Every eligible position within a deal gets equal selection probability.
	rng := rand.New(rand.NewSource(seed ^ 0x37a54e))
	turn := 0
	_, err = arena.Play(st, func(s game.GameState) ([]int, error) {
		n := len(s.Seats[s.Trick.TurnSeat].Hand)
		if n >= minHand && n <= maxHand {
			d.Eligible++
			if rng.Intn(d.Eligible) == 0 {
				d.State = s
				d.PlayIndex = turn
			}
		}
		turn++
		return p.play(s)
	}, nil)
	if err != nil || d.Eligible == 0 {
		return d, err
	}
	d.Table = bot.Observe(d.State, d.State.Trick.TurnSeat)
	d.Choices, d.Candidates, err = rolloutShortlist(d.State, d.Table, visitor, rng, false, lateTop3)
	return d, err
}

func rolloutShortlist(st game.GameState, table bot.Table, visitor *bot.LearnModel, rng *rand.Rand, skipForced, lateTop3 bool) ([]endgame.Choice, int, error) {
	// A candidate is a distinct nonempty subset of at most ten cards.
	ranked, err := visitor.Rank(table, 1<<10)
	if err != nil {
		return nil, 0, err
	}
	var choices []endgame.Choice
	var ranks []int
	if lateTop3 {
		for i := range min(len(ranked), 3) {
			ranks = append(ranks, i)
		}
	} else {
		ranks = branchRanks(len(ranked), rng)
	}
	for _, r := range ranks {
		cards, e := ids(ranked[r].Action)
		if e != nil {
			return nil, 0, e
		}
		var value int
		if !skipForced || len(ranked) > 1 {
			value, e = branchValue(st, cards, visitor)
			if e != nil {
				return nil, 0, e
			}
		}
		choices = append(choices, endgame.Choice{Cards: game.PlayCardsPayload{CardIDs: cards}, NetLevels: value})
	}
	return choices, len(ranked), nil
}

// Midroll samples alternatives only from information already in this state.
// Each hypothetical trajectory uses raw on all four seats. Replanning occurs
// only once the actual game advances to its next decision.
func midrollPlay(st game.GameState, m *bot.LearnModel) ([]int, error) {
	seat := st.Trick.TurnSeat
	hand := st.Seats[seat].Hand
	if len(hand) < 6 || len(hand) > 10 {
		return (policy{m, "raw"}).play(st)
	}
	seed := int64(st.TrickIndex*4 + seat)
	for _, c := range hand {
		seed = seed*109 + int64(c.ID+1)
	}
	options, _, err := rolloutShortlist(st, bot.Observe(st, seat), m, rand.New(rand.NewSource(seed)), true, false)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("no rollout candidate")
	}
	best := options[0]
	for _, opt := range options[1:] {
		if opt.NetLevels > best.NetLevels {
			best = opt
		}
	}
	return best.Cards.CardIDs, nil
}

type calibrationCounts struct {
	Positions, Branches, Disagree, AbsoluteError, SignedError                        int
	ExactInformative, RolloutInformative, BothInformative                            int
	InformativeTopOptimal, InformativeTopRegret, TiedRolloutWithInferior             int
	ExactSingleBest, StrictPairs, PairOrderCorrect, PairOrderTied, PairOrderReversed int
}

// Restrict the student's normal greedy ordering to the reference shortlist.
func rerankLate(table bot.Table, ref, student *bot.LearnModel) ([]int, error) {
	options, err := ref.Rank(table, 3)
	if err != nil {
		return nil, err
	}
	if len(options) == 1 {
		return ids(options[0].Action)
	}
	// No more than 63 nonempty subsets of a six-card hand.
	ranked, err := student.Rank(table, 63)
	if err != nil {
		return nil, err
	}
	shortlist := make(map[[2]uint64]bool, len(options))
	for _, r := range options {
		cards, err := ids(r.Action)
		if err != nil {
			return nil, err
		}
		key, err := actionSet(cards)
		if err != nil {
			return nil, err
		}
		shortlist[key] = true
	}
	for _, r := range ranked {
		cards, err := ids(r.Action)
		if err != nil {
			return nil, err
		}
		key, err := actionSet(cards)
		if err != nil {
			return nil, err
		}
		if shortlist[key] {
			return cards, nil
		}
	}
	return nil, fmt.Errorf("student has no candidate on the reference shortlist")
}

// Compare the same three candidates as normal search, but play each to
// settlement when the actor holds four to six cards. Rank breaks payoff ties.
func lateRollPlay(st game.GameState, m *bot.LearnModel) ([]int, error) {
	seat := st.Trick.TurnSeat
	if n := len(st.Seats[seat].Hand); n < 4 || n > 6 {
		return (policy{m, "search"}).play(st)
	}
	ranked, err := m.Rank(bot.Observe(st, seat), 3)
	if err != nil {
		return nil, err
	}
	if len(ranked) == 0 {
		return nil, fmt.Errorf("no ranked action for seat %d", seat)
	}
	if len(ranked) == 1 {
		return ids(ranked[0].Action)
	}
	var best []int
	var bestValue int
	for _, candidate := range ranked {
		cards, err := ids(candidate.Action)
		if err != nil {
			return nil, err
		}
		value, err := branchValue(st, cards, m)
		if err != nil {
			return nil, err
		}
		if best == nil || value > bestValue {
			best, bestValue = cards, value
		}
	}
	return best, nil
}

// The reference ranking breaks rollout ties for the top action. Include every
// distinct engine intent in the <=3-card audit, even ones omitted by the actor.
func calibrateDeal(seed int64, visitor *bot.LearnModel) (calibrationCounts, error) {
	var c calibrationCounts
	st, err := arena.NewRound(seed)
	if err != nil {
		return c, err
	}
	p := policy{visitor, "raw"}
	_, err = arena.Play(st, func(s game.GameState) ([]int, error) {
		play, e := p.play(s)
		if e != nil || !endgame.Eligible(s) || len(s.Seats[s.Trick.TurnSeat].Hand) == 1 {
			return play, e
		}
		all, e := endgame.Choices(s)
		if e != nil || len(all) < 2 {
			return play, e
		}
		exact, rolled := make([]int, len(all)), make([]int, len(all))
		rootKey, e := actionSet(play)
		if e != nil {
			return nil, e
		}
		rootIndex := -1
		for i, opt := range all {
			exact[i] = opt.NetLevels
			rolled[i], e = branchValue(s, opt.Cards.CardIDs, visitor)
			if e != nil {
				return nil, e
			}
			key, e := actionSet(opt.Cards.CardIDs)
			if e != nil {
				return nil, e
			}
			if key == rootKey {
				rootIndex = i
			}
		}
		if rootIndex < 0 {
			return nil, fmt.Errorf("visitor play %v missing from exact solver", play)
		}
		c.record(exact, rolled, rootIndex)
		return play, nil
	}, nil)
	return c, err
}

func actionSet(cards []int) ([2]uint64, error) {
	var k [2]uint64
	for _, id := range cards {
		if id < 0 || id >= 108 {
			return k, fmt.Errorf("invalid card ID %d", id)
		}
		k[id/64] |= 1 << uint(id%64)
	}
	return k, nil
}

func (c *calibrationCounts) record(exact, rolled []int, root int) {
	c.Positions++
	c.Branches += len(exact)
	top := root
	best, worst := exact[0], exact[0]
	minRolled := rolled[0]
	for i, v := range exact {
		diff := rolled[i] - v
		c.AbsoluteError += max(diff, -diff)
		c.SignedError += diff
		if diff != 0 {
			c.Disagree++
		}
		best = max(best, v)
		worst = min(worst, v)
		minRolled = min(minRolled, rolled[i])
		if rolled[i] > rolled[top] {
			top = i
		}
		for j := 0; j < i; j++ {
			if v == exact[j] {
				continue
			}
			c.StrictPairs++
			order := (v - exact[j]) * (rolled[i] - rolled[j])
			switch {
			case order > 0:
				c.PairOrderCorrect++
			case order == 0:
				c.PairOrderTied++
			default:
				c.PairOrderReversed++
			}
		}
	}
	informative := rolled[top] > minRolled
	if informative {
		c.RolloutInformative++
	}
	if best == worst {
		return
	}
	c.ExactInformative++
	if informative {
		c.BothInformative++
	}
	if exact[top] == best {
		c.InformativeTopOptimal++
	}
	c.InformativeTopRegret += best - exact[top]
	bestCount := 0
	unsafeTie := false
	for i, v := range exact {
		if v == best {
			bestCount++
		}
		if i != top && rolled[i] == rolled[top] && v != exact[top] {
			unsafeTie = true
		}
	}
	if unsafeTie {
		c.TiedRolloutWithInferior++
	}
	if bestCount == 1 {
		c.ExactSingleBest++
	}
}

func calibrate(args []string) error {
	f := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	path := f.String("model", "training/teacher/best.json", "frozen visitor and continuation")
	start := f.Int64("seed", 43000000, "first audit deal")
	count := f.Int("deals", 512, "number of audit deals")
	workers := f.Int("workers", 6, "parallel deals")
	out := f.String("out", "training/branch/calibration.json", "report path")
	if err := f.Parse(args); err != nil {
		return err
	}
	m, err := load(*path)
	if err != nil {
		return err
	}
	deals, err := parallel(*count, *workers, func(i int) (calibrationCounts, error) { return calibrateDeal(*start+int64(i), m) })
	if err != nil {
		return err
	}
	var t calibrationCounts
	for _, d := range deals {
		t.Positions += d.Positions
		t.Branches += d.Branches
		t.Disagree += d.Disagree
		t.AbsoluteError += d.AbsoluteError
		t.SignedError += d.SignedError
		t.ExactInformative += d.ExactInformative
		t.RolloutInformative += d.RolloutInformative
		t.BothInformative += d.BothInformative
		t.InformativeTopOptimal += d.InformativeTopOptimal
		t.InformativeTopRegret += d.InformativeTopRegret
		t.TiedRolloutWithInferior += d.TiedRolloutWithInferior
		t.ExactSingleBest += d.ExactSingleBest
		t.StrictPairs += d.StrictPairs
		t.PairOrderCorrect += d.PairOrderCorrect
		t.PairOrderTied += d.PairOrderTied
		t.PairOrderReversed += d.PairOrderReversed
	}
	if _, err = save(*out, map[string]any{"model": *path, "seed": *start, "deals": *count, "metrics": t, "rootTieBreak": "visitor's play if among top rollout values, else first solver action with top rollout value", "population": "visited positions where all hands have <=3 cards and the root has >=2 legal card-ID sets; all root intents included"}); err != nil {
		return err
	}
	fmt.Printf("exactly discriminating positions %d; rollout top optimal %d; unequal branch values %d/%d; strict pairs correct/tied/reversed %d/%d/%d\n", t.ExactInformative, t.InformativeTopOptimal, t.Disagree, t.Branches, t.PairOrderCorrect, t.PairOrderTied, t.PairOrderReversed)
	return nil
}
