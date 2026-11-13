package arena

import (
	"reflect"
	"testing"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func playSingle(st game.GameState) ([]int, error) {
	hand := st.Seats[st.Trick.TurnSeat].Hand
	if lead := st.Trick.Plays[st.Trick.LeaderSeat]; lead != nil {
		for _, c := range hand {
			if c.SuitClass == lead.SuitClass {
				return []int{c.ID}, nil
			}
		}
	}
	return []int{hand[0].ID}, nil
}

func TestReplayReproducesSeededRound(t *testing.T) {
	for _, seed := range []int64{3, 11, 27} {
		st, err := NewRound(seed)
		if err != nil {
			t.Fatal(err)
		}
		if len(st.Bottom) != 8 {
			t.Fatalf("seed %d has %d bottom cards", seed, len(st.Bottom))
		}
		for seat, s := range st.Seats {
			if len(s.Hand) != 25 {
				t.Fatalf("seed %d seat %d has %d cards", seed, seat, len(s.Hand))
			}
		}
		trace := Replay{Seed: seed}
		final, err := Play(st, playSingle, &trace)
		if err != nil {
			t.Fatal(err)
		}
		replayed, err := trace.Verify()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(final, replayed) || len(trace.Moves) != 100 || Reward(final, 1) != -Reward(final, 0) {
			t.Fatalf("seed %d replay or team reward mismatch", seed)
		}
		trace.Moves[0].Seat = (trace.Moves[0].Seat + 1) % 4
		if _, err := trace.Verify(); err == nil {
			t.Fatal("corrupt replay accepted")
		}
	}
}

func TestFullMatchCarriesSettlementToNextDeal(t *testing.T) {
	next := 0
	teams := [2]game.TeamState{{LevelRank: rules.R2}, {LevelRank: rules.R2}}
	starter := 0
	final, err := Match(138713, func(st game.GameState) ([]int, error) {
		if st.TrickIndex == 0 && st.Trick.TurnSeat == st.Trick.LeaderSeat {
			if st.RoundIndex != next || st.Teams != teams || st.NextStarterSeat != starter {
				t.Fatalf("did not carry settlement into round %d", next)
			}
			mode := game.CallModeOrdered
			if next == 0 {
				mode = game.CallModeRace
			}
			if st.CallMode != mode {
				t.Fatalf("round %d calling mode %s", next, st.CallMode)
			}
		}
		ids, err := playSingle(st)
		// Single-card policy always ends the 25th trick after its fourth play.
		if st.TrickIndex == 24 && (st.Trick.LeaderSeat+3)%4 == st.Trick.TurnSeat {
			settled, err := Step(st, ids)
			if err != nil {
				t.Fatal(err)
			}
			teams, starter = settled.Teams, settled.NextStarterSeat
			next++
		}
		return ids, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if final.Phase != game.PhaseGameOver || final.Teams[final.WinnerTeam].LevelRank != rules.RA || next != final.RoundIndex+1 {
		t.Fatal("match did not reach the winning level")
	}
}
