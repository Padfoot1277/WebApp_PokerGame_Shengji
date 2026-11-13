package main

import (
	"slices"
	"testing"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
)

// The training score must choose exactly the move made by the actual search,
// including sign reversal when a candidate hands the next lead to an opponent.
func TestLeafScoresSelectSearchMove(t *testing.T) {
	actor, value := bot.NewLearnModel(82), bot.NewLearnModel(99)
	st, err := arena.NewRound(13100)
	if err != nil {
		t.Fatal(err)
	}
	compared, flipped := 0, 0
	raw := policy{actor, "raw"}
	_, err = arena.Play(st, func(s game.GameState) ([]int, error) {
		seat := s.Trick.TurnSeat
		if endgame.Eligible(s) {
			return raw.play(s)
		}
		ranked, e := actor.Rank(bot.Observe(s, seat), 3)
		if e != nil {
			return nil, e
		}
		row := make([]bot.LeafTarget, len(ranked))
		for i, move := range ranked {
			next, e := bot.SearchLeaf(s, move.Action, actor)
			if e != nil {
				return nil, e
			}
			if next.Phase != game.PhasePlayTrick {
				row[i] = bot.LeafTarget{Levels: arena.Reward(next, s.Seats[seat].Team)}
				continue
			}
			leader := next.Trick.TurnSeat
			same := next.Seats[leader].Team == s.Seats[seat].Team
			if !same {
				flipped++
			}
			row[i] = bot.NextTrickTarget(bot.Observe(next, leader), same, 0)
		}
		actual, e := bot.SearchWithModels(s, actor, actor, value)
		if e != nil {
			return nil, e
		}
		want, e := ids(actual)
		if e != nil {
			return nil, e
		}
		got, e := ids(ranked[value.PickLeaf(row)].Action)
		if e != nil {
			return nil, e
		}
		if !slices.Equal(want, got) {
			t.Fatalf("expected %v got %v", want, got)
		}
		if len(row) > 1 {
			compared++
		}
		return raw.play(s)
	}, nil)
	if err != nil || compared == 0 || flipped == 0 {
		t.Fatalf("comparisons %d flips %d error %v", compared, flipped, err)
	}
}
