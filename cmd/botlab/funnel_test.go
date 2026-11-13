package main

import (
	"fmt"
	"testing"

	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestFunnelCollapsedThrowMatchesDirectRollout(t *testing.T) {
	deck := rules.NewDoubleDeck()
	st := game.GameState{Phase: game.PhasePlayTrick, CallerSeat: 0, BottomOwnerSeat: 0,
		Trump: game.TrumpState{Trump: rules.Trump{LevelRank: rules.R2, Suit: rules.Heart, HasTrumpSuit: true}},
		Trick: game.TrickState{LeaderSeat: 0, TurnSeat: 0, BiggerSeat: -1}, Points: 65,
		Bottom: []rules.Card{deck[4], deck[17]}}
	for i, h := range [4][4]int{{0, 54, 2, 40}, {1, 10, 33, 32}, {8, 9, 29, 31}, {6, 7, 43, 44}} {
		st.Seats[i] = game.SeatState{UID: fmt.Sprint(i), Team: i % 2, HandCount: 4}
		for _, id := range h {
			st.Seats[i].Hand = append(st.Seats[i].Hand, deck[id])
		}
		st.Teams[i%2].LevelRank = rules.R2
	}
	m := bot.NewLearnModel(131)
	s, err := enumerateFunnel(st, m)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := actionSet([]int{0, 54, 2})
	a := s.byIntent[k]
	if len(a.Actual) != 1 || a.Actual[0] != 2 || s.distinct >= len(s.byIntent) {
		t.Fatalf("AAQ did not collapse to Q: %+v", a)
	}
	for _, a := range s.byIntent {
		v, e := branchValue(st, a.Intent, m)
		if e != nil || v != a.Value {
			t.Fatalf("intent %v got %d expected %d, error %v", a.Intent, a.Value, v, e)
		}
	}
	score, err := scoreFunnel(st, bot.Observe(st, 0), s, m)
	if err != nil {
		t.Fatal(err)
	}
	if s.best.Value < score.Generated || score.Generated < score.Top3 || score.Top3 < score.Selected {
		t.Fatal(score)
	}
}
func TestFunnelLossDecomposition(t *testing.T) {
	var f funnelLoss
	f.add(3, funnelScore{Generated: 2, Top3: 1, Raw: 0, Selected: 0})
	f.add(3, funnelScore{Generated: 3, Top3: 1, Raw: 1, Selected: 1})
	f.add(3, funnelScore{Generated: 3, Top3: 3, Raw: 3, Selected: 1})
	f.add(3, funnelScore{Generated: 3, Top3: 3, Raw: 0, Selected: 3})
	if f.GenerationMiss != 1 || f.Top3Miss != 1 || f.SelectionMiss != 1 || f.Optimal != 1 || f.RawOptimal != 1 || f.GenerationGap+f.RankingGap+f.SelectionGap != 7 {
		t.Fatal(f)
	}
}
