package main

import (
	"math/rand"
	"slices"
	"testing"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
)

func TestBranchRankCoverage(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for n := 1; n <= 40; n++ {
		got := branchRanks(n, rng)
		if len(got) != min(n, 4) || got[0] != 0 || !slices.IsSorted(got) || got[len(got)-1] >= n {
			t.Fatalf("selected %v out of %d", got, n)
		}
		for i := 1; i < len(got); i++ {
			if got[i] == got[i-1] {
				t.Fatalf("duplicate %v", got)
			}
		}
	}
}

func TestRecordedBranchValueMatchesUninterruptedRound(t *testing.T) {
	m := bot.NewLearnModel(731)
	const seed = 700003
	d, err := collectBranchDeal(m, seed, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Eligible == 0 || len(d.Choices) == 0 || len(d.Choices) > 4 {
		t.Fatalf("missing decision on seed %d", seed)
	}
	seat := d.State.Trick.TurnSeat
	if n := len(d.State.Seats[seat].Hand); n < 6 || n > 10 {
		t.Fatalf("sampled hand size %d", n)
	}
	st, err := arena.NewRound(seed)
	if err != nil {
		t.Fatal(err)
	}
	final, err := arena.Play(st, (policy{m, "raw"}).play, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := arena.Reward(final, d.State.Seats[seat].Team)
	if d.Choices[0].NetLevels != want {
		t.Fatalf("top branch = %d, actual %d", d.Choices[0].NetLevels, want)
	}
}

func TestCalibrationCountsIncludeRolloutTies(t *testing.T) {
	var c calibrationCounts
	c.record([]int{0, 1, 2}, []int{1, 1, 0}, 0)
	if c.StrictPairs != 3 || c.PairOrderTied != 1 || c.PairOrderReversed != 2 ||
		c.InformativeTopRegret != 2 || c.Disagree != 2 || c.AbsoluteError != 3 {
		t.Fatalf("incorrect audit counts %+v", c)
	}
}

func TestLateRerankMatchesTrainingShortlist(t *testing.T) {
	ref := bot.NewLearnModel(731)
	student := bot.NewLearnModel(480)
	checked := 0
	for seed := int64(700003); seed < 700200 && checked < 2; seed++ {
		d, err := collectBranchDeal(ref, seed, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Choices) == 0 {
			continue
		}
		n := len(d.State.Seats[d.State.Trick.TurnSeat].Hand)
		if n < 4 || n > 6 || len(d.Choices) != min(3, d.Candidates) {
			t.Fatalf("hand %d, choices %d of %d", n, len(d.Choices), d.Candidates)
		}
		sample, err := ref.LabelRollouts(d.Table, d.Choices)
		if err != nil {
			t.Fatal(err)
		}
		regret, err := student.ExactRegret(sample)
		if err != nil {
			t.Fatal(err)
		}
		move, err := rerankLate(d.Table, ref, student)
		if err != nil {
			t.Fatal(err)
		}
		best := d.Choices[0].NetLevels
		var chosen *int
		for i := range d.Choices {
			c := &d.Choices[i]
			best = max(best, c.NetLevels)
			if slices.Equal(move, c.Cards.CardIDs) {
				chosen = &c.NetLevels
			}
		}
		if chosen == nil || best-*chosen != regret {
			t.Fatalf("move %v does not reproduce training regret %d", move, regret)
		}
		if sample.Informative() {
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no unequal branch payoffs checked")
	}
}

func TestMidrollPreservesReferenceContinuationValue(t *testing.T) {
	m := bot.NewLearnModel(731)
	for seed := int64(700003); seed < 700009; seed++ {
		d, err := collectBranchDeal(m, seed, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Choices) == 0 {
			continue
		}
		got, err := (policy{m, "midroll"}).play(d.State)
		if err != nil {
			t.Fatal(err)
		}
		again, err := midrollPlay(d.State, m)
		if err != nil || !slices.Equal(got, again) {
			t.Fatalf("midroll nondeterministic %v vs %v: %v", got, again, err)
		}
		v, err := branchValue(d.State, got, m)
		if err != nil {
			t.Fatal(err)
		}
		orig := d.Choices[0]
		if v < orig.NetLevels || v == orig.NetLevels && !slices.Equal(got, orig.Cards.CardIDs) {
			t.Fatalf("rollout worsened fixed continuation or broke tie: %v vs %+v", got, orig)
		}
	}
	st, err := arena.NewRound(700003)
	if err != nil {
		t.Fatal(err)
	}
	out, err := midrollPlay(st, m)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := (policy{m, "raw"}).play(st)
	if err != nil || !slices.Equal(out, raw) {
		t.Fatalf("full-hand opening %v differs from raw %v, %v", out, raw, err)
	}
}
