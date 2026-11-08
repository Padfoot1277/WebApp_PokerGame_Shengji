package bot

import (
	"testing"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func card(id int, r rules.Rank) rules.Card {
	return rules.Card{ID: id, Rank: r, Suit: rules.Heart, SuitClass: rules.SCH}
}
func TestLeadTractor(t *testing.T) {
	h := []rules.Card{card(1, rules.RQ), card(55, rules.RQ), card(2, rules.RK), card(56, rules.RK), card(3, rules.R3)}
	s := game.ViewState{MyHand: [][]rules.Card{h}, MySeat: 0}
	s.Trump.LevelRank = rules.R2
	got, err := choose(s)
	if err != nil || len(got) != 4 {
		t.Fatalf("want tractor, got %v, %v", got, err)
	}
}
func TestFollowTeamAndPoints(t *testing.T) {
	for _, partner := range []bool{false, true} {
		s := game.ViewState{MySeat: 3, MyHand: [][]rules.Card{{card(2, rules.R3), card(3, rules.R10), card(4, rules.RA)}}}
		s.Trump.LevelRank = rules.R2
		lead := card(1, rules.RK)
		blocks, _ := rules.DecomposeThrow([]rules.Card{lead}, s.Trump.Trump, rules.SCH)
		s.Trick.LeaderSeat = 0
		s.Trick.BiggerSeat = 0
		s.Trick.Plays[0] = &game.PlayedMove{SuitClass: rules.SCH, Move: game.Move{Cards: []rules.Card{lead}, Blocks: blocks}}
		s.Seats[3].Team = 1
		if partner {
			s.Seats[0].Team = 1
		}
		got, err := choose(s)
		if err != nil {
			t.Fatal(err)
		}
		want := rules.RA
		if partner {
			want = rules.R10
		}
		if got[0].Rank != want {
			t.Fatalf("partner=%v: want %s got %v", partner, want, got)
		}
		if err := rules.ValidateHideCheck(s.MyHand[0], got, blocks, s.Trump.Trump); err != nil {
			t.Fatal(err)
		}
	}
}
