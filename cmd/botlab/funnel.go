package main

import (
	"flag"
	"fmt"
	"math/rand"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
)

type funnelAction struct {
	Intent, Actual []int
	Value          int
}

type funnelSpace struct {
	byIntent    map[[2]uint64]funnelAction
	distinct    int
	best, worst funnelAction
}

// The actor's remaining entities determine the cards actually submitted after
// the server resolves a failed throw. The frozen actor/critic never observes
// the clipped intent or throw notification. Do not merge different deck copies:
// a later deterministic tie break can depend on the surviving entity IDs.
func enumerateFunnel(st game.GameState, visitor *bot.LearnModel) (funnelSpace, error) {
	s := funnelSpace{byIntent: make(map[[2]uint64]funnelAction)}
	seat := st.Trick.TurnSeat
	hand := st.Seats[seat].Hand
	if len(hand) < 4 || len(hand) > 6 {
		return s, fmt.Errorf("expected actor to hold 4..6 cards")
	}
	cache := map[[2]uint64]int{}
	play := (policy{visitor, "raw"}).play
	for mask := 1; mask < 1<<len(hand); mask++ {
		var cards []int
		for i, c := range hand {
			if mask&(1<<i) != 0 {
				cards = append(cards, c.ID)
			}
		}
		next, err := arena.Step(st, cards)
		if err != nil {
			continue
		} // Engine rejected this subset.
		var actual []int
		for _, id := range cards {
			taken := true
			for _, c := range next.Seats[seat].Hand {
				if c.ID == id {
					taken = false
					break
				}
			}
			if taken {
				actual = append(actual, id)
			}
		}
		if len(actual) == 0 {
			return s, fmt.Errorf("legal intent removed no cards")
		}
		key, err := actionSet(actual)
		if err != nil {
			return s, err
		}
		v, cached := cache[key]
		if !cached {
			final, err := arena.Play(next, play, nil)
			if err != nil {
				return s, err
			}
			v = arena.Reward(final, st.Seats[seat].Team)
			cache[key] = v
		}
		a := funnelAction{cards, actual, v}
		intent, err := actionSet(cards)
		if err != nil {
			return s, err
		}
		if len(s.byIntent) == 0 || v > s.best.Value {
			s.best = a
		}
		if len(s.byIntent) == 0 || v < s.worst.Value {
			s.worst = a
		}
		s.byIntent[intent] = a
	}
	s.distinct = len(cache)
	if s.distinct == 0 {
		return s, fmt.Errorf("no accepted intents")
	}
	return s, nil
}

func (s funnelSpace) lookup(a bot.Action) (funnelAction, error) {
	cards, err := ids(a)
	if err != nil {
		return funnelAction{}, err
	}
	k, err := actionSet(cards)
	if err != nil {
		return funnelAction{}, err
	}
	o, ok := s.byIntent[k]
	if !ok {
		return o, fmt.Errorf("policy action %v absent from exhaustive legal intents", cards)
	}
	return o, nil
}

type funnelScore struct {
	Generated, Top3, Raw, Selected int
	SelectedCards                  []int
}

func scoreFunnel(st game.GameState, table bot.Table, s funnelSpace, m *bot.LearnModel) (funnelScore, error) {
	ranked, err := m.Rank(table, len(s.byIntent))
	if err != nil {
		return funnelScore{}, err
	}
	res := funnelScore{Generated: s.worst.Value, Top3: s.worst.Value}
	for i, r := range ranked {
		o, e := s.lookup(r.Action)
		if e != nil {
			return res, e
		}
		res.Generated = max(res.Generated, o.Value)
		if i < 3 {
			res.Top3 = max(res.Top3, o.Value)
		}
		if i == 0 {
			res.Raw = o.Value
		}
	}
	a, err := bot.LearnedPlay(st, m)
	if err != nil {
		return res, err
	}
	o, err := s.lookup(a)
	if err != nil {
		return res, err
	}
	res.Selected = o.Value
	res.SelectedCards = o.Intent
	if res.Top3 < res.Selected {
		return res, fmt.Errorf("search selected outside top three")
	}
	return res, nil
}

type funnelDeal struct {
	groups                       []string
	legal, distinct, best, worst int
	scores                       [2]funnelScore
	swapValues                   [3]int // early rank, early reply, or early leaf; others late
	example                      *midDetail
	bestIntent, bestActual       string
	seed                         int64
}

func funnelSlice(st game.GameState, table bot.Table) []string {
	var position string
	lead := st.Trick.Plays[st.Trick.LeaderSeat]
	switch {
	case lead == nil:
		position = "lead"
	case len(lead.Cards) == 1:
		position = "followSingle"
	default:
		position = "followMulti"
	}
	groups := []string{"all", position}
	if lead != nil && st.Trick.BiggerSeat == (st.Trick.TurnSeat+2)%4 {
		groups = append(groups, "partnerAhead")
	}
	if table.CanDig {
		groups = append(groups, "canDig")
	} else {
		groups = append(groups, "noDig")
	}
	for _, target := range []int{5, 40, 80, 120, 160, 200} {
		if st.Points < target && target-st.Points <= 10 {
			groups = append(groups, "within10BelowThreshold")
			break
		}
	}
	return groups
}

func analyzeFunnel(seed int64, visitor, base, candidate *bot.LearnModel, swaps bool) (funnelDeal, error) {
	d := funnelDeal{seed: seed}
	st, err := arena.NewRound(seed)
	if err != nil {
		return d, err
	}
	rng := rand.New(rand.NewSource(seed ^ 0x47b212))
	var root game.GameState
	k := 0
	_, err = arena.Play(st, func(s game.GameState) ([]int, error) {
		if n := len(s.Seats[s.Trick.TurnSeat].Hand); n >= 4 && n <= 6 {
			k++
			if rng.Intn(k) == 0 {
				root = s
			}
		}
		return (policy{visitor, "raw"}).play(s)
	}, nil)
	if err != nil || k == 0 {
		return d, err
	}
	table := bot.Observe(root, root.Trick.TurnSeat)
	s, err := enumerateFunnel(root, visitor)
	if err != nil {
		return d, err
	}
	d.groups = funnelSlice(root, table)
	if root.Trick.Plays[root.Trick.LeaderSeat] == nil {
		intent, err := (policy{visitor, "raw"}).play(root)
		if err != nil {
			return d, err
		}
		if len(intent) > 1 {
			d.groups = append(d.groups, "leadVisitorMulti")
		}
	}
	d.legal, d.distinct, d.best, d.worst = len(s.byIntent), s.distinct, s.best.Value, s.worst.Value
	if d.best == d.worst {
		return d, nil
	}
	for i, m := range []*bot.LearnModel{base, candidate} {
		d.scores[i], err = scoreFunnel(root, table, s, m)
		if err != nil {
			return d, err
		}
	}
	if swaps {
		for i := range d.swapValues {
			parts := [3]*bot.LearnModel{candidate, candidate, candidate}
			parts[i] = base
			action, e := bot.SearchWithModels(root, parts[0], parts[1], parts[2])
			if e != nil {
				return d, e
			}
			move, e := s.lookup(action)
			if e != nil {
				return d, e
			}
			d.swapValues[i] = move.Value
		}
	}
	d.example = describeMid(root, d.scores[0].SelectedCards, d.scores[1].SelectedCards)
	d.bestIntent = midCards(root, s.best.Intent)
	d.bestActual = midCards(root, s.best.Actual)
	return d, nil
}

type funnelLoss struct {
	GenerationMiss, Top3Miss, SelectionMiss, Optimal int
	RawOptimal                                       int
	GenerationGap, RankingGap, SelectionGap, RawGap  float64
}

func (f *funnelLoss) add(best int, s funnelScore) {
	switch {
	case s.Generated < best:
		f.GenerationMiss++
	case s.Top3 < best:
		f.Top3Miss++
	case s.Selected < best:
		f.SelectionMiss++
	default:
		f.Optimal++
	}
	if s.Raw == best {
		f.RawOptimal++
	}
	f.GenerationGap += float64(best - s.Generated)
	f.RankingGap += float64(s.Generated - s.Top3)
	f.SelectionGap += float64(s.Top3 - s.Selected)
	f.RawGap += float64(best - s.Raw)
}

type funnelGroup struct {
	Roots, Forced, AllTie, Informative, LegalIntents, DistinctActual int
	Models                                                           [2]funnelLoss
}

func funnel(args []string) error {
	f := flag.NewFlagSet("funnel", flag.ContinueOnError)
	b := f.String("baseline", "training/teacher/best.json", "first evaluated checkpoint")
	v := f.String("visitor", "", "frozen visitor and common continuation (default baseline)")
	m := f.String("model", "", "second model (default baseline)")
	n := f.Int("deals", 1024, "independent deals")
	seed := f.Int64("seed", 64000000, "first deal seed")
	w := f.Int("workers", 8, "parallel deals")
	out := f.String("out", "training/funnel.json", "report")
	swaps := f.Bool("swaps", false, "evaluate one-at-a-time search component swaps from baseline into model on identical roots")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *m == "" {
		*m = *b
	}
	if *v == "" {
		*v = *b
	}
	base, err := load(*b)
	if err != nil {
		return err
	}
	candidate, err := load(*m)
	if err != nil {
		return err
	}
	visitor, err := load(*v)
	if err != nil {
		return err
	}
	deals, err := parallel(*n, *w, func(i int) (funnelDeal, error) {
		return analyzeFunnel(*seed+int64(i), visitor, base, candidate, *swaps)
	})
	if err != nil {
		return err
	}
	groups := map[string]*funnelGroup{}
	examples := map[string]any{}
	type swapOutcome struct{ Better, Worse, Equal, NetLevels int }
	outcomes := map[string]*swapOutcome{"rank": {}, "reply": {}, "value": {}}
	for _, d := range deals {
		for _, name := range d.groups {
			g := groups[name]
			if g == nil {
				g = &funnelGroup{}
				groups[name] = g
			}
			g.Roots++
			g.LegalIntents += d.legal
			g.DistinctActual += d.distinct
			switch {
			case d.distinct == 1:
				g.Forced++
			case d.best == d.worst:
				g.AllTie++
			default:
				g.Informative++
				for i, s := range d.scores {
					g.Models[i].add(d.best, s)
				}
			}
		}
		if d.example != nil {
			if *swaps {
				for i, key := range []string{"rank", "reply", "value"} {
					delta := d.swapValues[i] - d.scores[1].Selected
					o := outcomes[key]
					o.NetLevels += delta
					switch {
					case delta > 0:
						o.Better++
					case delta < 0:
						o.Worse++
					default:
						o.Equal++
					}
				}
			}
			s := d.scores[0]
			kind := "success"
			switch {
			case s.Generated < d.best:
				kind = "generationMiss"
			case s.Top3 < d.best:
				kind = "top3Miss"
			case s.Selected < d.best:
				kind = "selectionMiss"
			}
			if _, ok := examples[kind]; !ok {
				examples[kind] = map[string]any{"seed": d.seed, "groups": d.groups, "best": d.best, "bestIntent": d.bestIntent, "actualCards": d.bestActual, "scores": d.scores, "table": d.example}
			}
		}
	}
	for _, g := range groups {
		if g.Informative > 0 {
			for i := range g.Models {
				l := &g.Models[i]
				count := float64(g.Informative)
				l.GenerationGap /= count
				l.RankingGap /= count
				l.SelectionGap /= count
				l.RawGap /= count
			}
		}
	}
	report := map[string]any{"deals": *n, "seed": *seed, "visitor": *v, "models": []string{*b, *m}, "groups": groups, "examples": examples, "metric": "One sampled 4-6 card acting position per deal. Target is signed net levels after fixed visitor raw continuation, not minimax. Stage miss = first removal of all root-best moves. Mean incremental gaps conditional on informative roots. Groups overlap."}
	if *swaps {
		report["componentSwapVsCandidateOnInformativeRoots"] = outcomes
	}
	_, err = save(*out, report)
	if err == nil && groups["all"] != nil {
		g := groups["all"]
		fmt.Printf("sampled %d/%d, informative %d, baseline/candidate %v\n", g.Roots, *n, g.Informative, g.Models)
	}
	return err
}
