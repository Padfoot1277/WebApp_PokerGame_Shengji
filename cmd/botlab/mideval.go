package main

import (
	"flag"
	"fmt"
	"math/rand"
	"slices"

	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// Compare freely chosen actions. A student may select an action outside the
// teacher's training shortlist. Both actions receive the same continuation.
type midDecision struct {
	Seed                 int64
	HasRoot, Changed     bool
	OldLevels, NewLevels int
	ShortlistBest        int
	Detail               *midDetail `json:",omitempty"`
}
type midDetail struct {
	Seat, Leader, Caller, Points int
	Trump                        rules.Trump
	Hands, Plays                 [4]string
	Old, New                     string
}

func midCards(st game.GameState, ids []int) string {
	var cards []rules.Card
	for _, c := range st.Seats[st.Trick.TurnSeat].Hand {
		if slices.Contains(ids, c.ID) {
			cards = append(cards, c)
		}
	}
	return cardNames(cards)
}

func describeMid(st game.GameState, old, new []int) *midDetail {
	d := midDetail{Seat: st.Trick.TurnSeat, Leader: st.Trick.LeaderSeat, Caller: st.CallerSeat, Points: st.Points, Trump: st.Trump.Trump, Old: midCards(st, old), New: midCards(st, new)}
	for i, s := range st.Seats {
		d.Hands[i] = cardNames(s.Hand)
		if p := st.Trick.Plays[i]; p != nil {
			d.Plays[i] = cardNames(p.Cards)
		}
	}
	return &d
}

func compareMid(seed int64, base, model *bot.LearnModel, lateTop3 bool) (midDecision, error) {
	r := midDecision{Seed: seed}
	b, err := collectBranchDeal(base, seed, lateTop3)
	if err != nil || len(b.Choices) == 0 {
		return r, err
	}
	r.HasRoot = true
	var cards []int
	if lateTop3 {
		cards, err = rerankLate(b.Table, base, model)
	} else {
		cards, err = (policy{model, "raw"}).play(b.State)
	}
	if err != nil {
		return r, err
	}
	old := b.Choices[0]
	r.OldLevels = old.NetLevels
	r.ShortlistBest = old.NetLevels
	for _, choice := range b.Choices {
		r.ShortlistBest = max(r.ShortlistBest, choice.NetLevels)
	}
	// A ranker may return the same entities in a different order.
	same := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for _, id := range a {
			if !slices.Contains(b, id) {
				return false
			}
		}
		return true
	}
	r.Changed = !same(old.Cards.CardIDs, cards)
	found := false
	for _, c := range b.Choices {
		if same(c.Cards.CardIDs, cards) {
			r.NewLevels, found = c.NetLevels, true
			break
		}
	}
	if !found {
		r.NewLevels, err = branchValue(b.State, cards, base)
		if err != nil {
			return r, err
		}
	}
	if r.NewLevels != r.OldLevels {
		r.Detail = describeMid(b.State, old.Cards.CardIDs, cards)
	}
	return r, nil
}

func mideval(args []string) error {
	f := flag.NewFlagSet("mideval", flag.ContinueOnError)
	basePath := f.String("baseline", "training/teacher/best.json", "frozen visiting and continuation policy")
	path := f.String("model", "training/midplay-run/best.json", "student weights")
	out := f.String("out", "training/midplay-run/heldout.json", "report file")
	count := f.Int("deals", 1536, "independent held-out deals")
	start := f.Int64("seed", 43000000, "first held-out deal")
	workers := f.Int("workers", 8, "parallel games")
	lateTop3 := f.Bool("late-top3", false, "compare student and baseline within the fixed late-game shortlist")
	if err := f.Parse(args); err != nil {
		return err
	}
	base, err := load(*basePath)
	if err != nil {
		return err
	}
	model, err := load(*path)
	if err != nil {
		return err
	}
	deals, err := parallel(*count, *workers, func(i int) (midDecision, error) { return compareMid(*start+int64(i), base, model, *lateTop3) })
	if err != nil {
		return err
	}
	var changed, better, worse, sum, available int
	var values []int
	haveGood, haveBad := false, false
	for i, r := range deals {
		if !r.HasRoot {
			continue
		}
		delta := r.NewLevels - r.OldLevels
		values = append(values, delta)
		sum += delta
		available += r.ShortlistBest - r.OldLevels
		if r.Changed {
			changed++
		}
		if delta > 0 {
			better++
		}
		if delta < 0 {
			worse++
		}
		if delta > 0 && !haveGood {
			haveGood = true
		} else if delta < 0 && !haveBad {
			haveBad = true
		} else {
			deals[i].Detail = nil
		}
	}
	if len(values) == 0 {
		return fmt.Errorf("no midgame roots")
	}
	rng := rand.New(rand.NewSource(90124))
	means := make([]float64, 10000)
	for i := range means {
		total := 0
		for range values {
			total += values[rng.Intn(len(values))]
		}
		means[i] = float64(total) / float64(len(values))
	}
	slices.Sort(means)
	mean := float64(sum) / float64(len(values))
	_, err = save(*out, map[string]any{"baseline": *basePath, "model": *path, "lateTop3": *lateTop3, "firstSeed": *start, "deals": *count, "roots": len(values), "changedActions": changed, "better": better, "worse": worse, "sumNetLevelGain": sum, "availableShortlistGain": available, "meanNetLevelGain": mean, "paired95": [2]float64{means[249], means[9749]}, "results": deals})
	if err == nil {
		fmt.Printf("%d roots, changed %d, better/worse %d/%d, net gain %.4f [%.4f, %.4f]\n", len(values), changed, better, worse, mean, means[249], means[9749])
	}
	return err
}
