package bot

import (
	"testing"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestAutomaticTrumpChoosesLongSuitAndAttacks(t *testing.T) {
	s := phaseState()
	s.MyHand[0] = append(s.MyHand[0], rules.Card{ID: 52, Suit: rules.SmallJoker, Rank: rules.RSJ}, rules.Card{ID: 12, Suit: rules.Spade, Rank: rules.R2})
	a, err := Decide(s, nil, true)
	if err != nil || a.Type != string(game.EvCallTrump) {
		t.Fatalf("%+v %v", a, err)
	}
	p := a.Payload.(game.CallTrumpPayload)
	if p.LevelIDs[0] != 25 || len(p.LevelIDs) != 2 {
		t.Fatalf("did not select long hearts: %+v", p)
	}
	s.Phase = game.PhaseTrumpFight
	s.FightTurnSeat = 0
	s.Trump.HasTrumpSuit = true
	s.Trump.Suit = rules.Spade
	s.Trump.Locked = true
	a, err = Decide(s, nil, true)
	if err != nil || a.Type != string(game.EvAttackTrump) {
		t.Fatalf("attack should beat change even when locked: %+v %v", a, err)
	}
	s.UsedTrumpIDs = []int{53, 107}
	a, err = Decide(s, nil, true)
	if err != nil || a.Type != string(game.EvCallPass) {
		t.Fatalf("used or locked cards accepted: %+v %v", a, err)
	}
}
func TestAutomaticBotsAllPhases(t *testing.T) {
	for run := 0; run < 8; run++ {
		st := game.GameState{Phase: game.PhaseLobby, CallerSeat: -1, BottomOwnerSeat: -1, CallMode: game.CallModeRace}
		for i := 0; i < 4; i++ {
			st.Seats[i] = game.SeatState{UID: string(rune('a' + i)), Team: i % 2, Online: true, IsBot: true}
			st.Teams[i%2].LevelRank = rules.R2
		}
		history := []PublicTrick{}
		steps := 0
		for st.Phase != game.PhaseRoundSettle && st.Phase != game.PhaseGameOver {
			progressed := false
			for seat := 0; seat < 4; seat++ {
				var a Action
				var err error
				if run%2 == 0 {
					a, err = DecideFull(Observe(st, seat), history)
				} else {
					a, err = Decide(game.MakeView(st, st.Seats[seat].UID), history, true)
				}
				if err != nil {
					t.Fatal(err)
				}
				if a.Type == "" {
					continue
				}
				index := st.TrickIndex
				res, err2 := game.Reduce(st, st.Seats[seat].UID, game.ClientEventType(a.Type), a.Payload)
				if err2 != nil {
					t.Fatalf("%s: %v", a.Type, err2)
				}
				st = res.State
				if st.TrickIndex > index {
					history = append(history, PublicTrick{st.Trick.LastLeaderSeat, st.Trick.LastPlays})
				}
				progressed = true
				steps++
				break
			}
			if !progressed || steps > 200 {
				t.Fatalf("bots stuck in %s after %d steps", st.Phase, steps)
			}
		}
		if st.Phase == game.PhaseRoundSettle {
			seat := st.NextStarterSeat
			var a Action
			var err error
			if run%2 == 0 {
				a, err = DecideFull(Observe(st, seat), history)
			} else {
				a, err = Decide(game.MakeView(st, st.Seats[seat].UID), history, true)
			}
			if err != nil || a.Type != string(game.EvStartNextRound) {
				t.Fatalf("next round: %+v %v", a, err)
			}
			res, err2 := game.Reduce(st, st.Seats[seat].UID, game.EvStartNextRound, a.Payload)
			if err2 != nil || res.State.RoundIndex != 1 || res.State.Phase != game.PhaseCallTrump {
				t.Fatalf("next round failed: %v", err2)
			}
		}
	}
}
