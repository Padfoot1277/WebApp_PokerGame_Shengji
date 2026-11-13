package endgame

import (
	"fmt"
	"testing"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestSaveAceToProtectBottom(t *testing.T) {
	// Two spade tricks. Leading the A first lets seat 1 cash K then dig
	// the 20-point bottom, taking the attackers from 60 to at least 80.
	deck := rules.NewDoubleDeck()
	st := game.GameState{Phase: game.PhasePlayTrick, CallerSeat: 0, BottomOwnerSeat: 0,
		Trump: game.TrumpState{Trump: rules.Trump{LevelRank: rules.R2, Suit: rules.Heart, HasTrumpSuit: true}},
		Trick: game.TrickState{LeaderSeat: 0, TurnSeat: 0, BiggerSeat: -1}, Points: 60,
		Bottom: []rules.Card{deck[4], deck[17]}}
	// A,3 vs K,4, with two non-scoring low cards at each remaining seat.
	for i, hand := range [4][2]int{{0, 11}, {1, 10}, {8, 9}, {6, 7}} {
		st.Seats[i] = game.SeatState{UID: fmt.Sprint(i), Team: i % 2, Hand: []rules.Card{deck[hand[0]], deck[hand[1]]}, HandCount: 2}
		st.Teams[i%2].LevelRank = rules.R2
	}
	opts, err := Choices(st)
	if err != nil {
		t.Fatal(err)
	}
	values := map[int]int{}
	for _, o := range opts {
		if len(o.Cards.CardIDs) == 1 {
			values[o.Cards.CardIDs[0]] = o.NetLevels
		}
	}
	if values[0] != 0 || values[11] != 1 {
		t.Fatalf("expected caller to retain a level by saving A, got %v", opts)
	}
	for team, want := range [2]int{1, -1} {
		v, err := Value(st, team)
		if err != nil || v != want {
			t.Fatalf("team %d value=%d want=%d err=%v", team, v, want, err)
		}
	}
	move, value, err := Best(st)
	if err != nil || value != 1 || len(move.CardIDs) != 1 || move.CardIDs[0] != 11 {
		t.Fatalf("best=%v value=%d err=%v", move, value, err)
	}
}
