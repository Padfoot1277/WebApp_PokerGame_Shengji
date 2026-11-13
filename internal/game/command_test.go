package game

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"upgrade-lan/internal/game/rules"
)

func fixture() GameState {
	st := GameState{Phase: PhasePlayTrick, CallerSeat: 0, BottomOwnerSeat: 0, WinnerTeam: -1, UsedTrumpIDs: map[int]bool{}, Trump: TrumpState{Trump: rules.Trump{LevelRank: rules.R2, HasTrumpSuit: true, Suit: rules.Heart}}}
	st.Teams = [2]TeamState{{rules.R2}, {rules.R2}}
	for i := range st.Seats {
		st.Seats[i] = SeatState{UID: fmt.Sprint(i), Team: i % 2, Online: true, Ready: true}
	}
	st.Trick = TrickState{LeaderSeat: 0, TurnSeat: 0, BiggerSeat: -1}
	return st
}
func TestInvalidCommandIsSafe(t *testing.T) {
	st := fixture()
	before := CloneState(st)
	for _, test := range []struct {
		typ ClientEventType
		p   any
	}{{EvReady, struct{}{}}, {EvPlayCards, struct{}{}}, {EvPlayCards, PlayCardsPayload{}}} {
		if _, err := Reduce(st, "0", test.typ, test.p); err == nil {
			t.Fatal("invalid command accepted")
		}
	}
	if !reflect.DeepEqual(st, before) {
		t.Fatal("rejected command mutated state")
	}
}
func TestSeatTeamInvariant(t *testing.T) {
	st := fixture()
	st.Phase = PhaseLobby
	st.Seats[1].UID = ""
	res, err := Reduce(st, "0", EvSit, SitPayload{1})
	if err != nil {
		t.Fatal(err)
	}
	st = res.State
	res, err = Reduce(st, "0", EvLeave, struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range res.State.Seats {
		if s.Team != i%2 {
			t.Fatalf("seat %d team %d", i, s.Team)
		}
	}
}
func TestSurrenderAndVictory(t *testing.T) {
	for _, points := range []int{0, 39, 40, 79, 80, 119, 120, 159, 160, 199, 200} {
		t.Run(fmt.Sprint(points), func(t *testing.T) {
			st := fixture()
			st.Points = points
			first, err := Reduce(st, "0", EvSurrender, struct{}{})
			if err != nil {
				t.Fatal(err)
			}
			if first.State.Phase != PhasePlayTrick {
				t.Fatal("single vote ended round")
			}
			second, err := Reduce(first.State, "2", EvSurrender, struct{}{})
			if err != nil {
				t.Fatal(err)
			}
			if second.State.Phase != PhaseRoundSettle || second.State.RoundPointsFinal != points || second.State.BottomAward != 0 {
				t.Fatalf("bad surrender %+v", second.State)
			}
		})
	}
	st := fixture()
	st.Teams[0].LevelRank = rules.RK
	st.Points = 40
	settleRoundEnd(&st)
	if st.Phase != PhaseGameOver || st.WinnerTeam != 0 {
		t.Fatal("reaching A must win")
	}
	if _, err := Reduce(st, "0", EvStartNextRound, struct{}{}); err == nil {
		t.Fatal("game over continued")
	}
}
func TestSurrenderCancelAndOpponents(t *testing.T) {
	st := fixture()
	r, _ := Reduce(st, "0", EvSurrender, nil)
	r, _ = Reduce(r.State, "1", EvSurrender, nil)
	if r.State.Phase != PhasePlayTrick {
		t.Fatal("opponents ended round")
	}
	r, _ = Reduce(r.State, "0", EvCancelSurrender, nil)
	if r.State.SurrenderMask != 2 {
		t.Fatal("vote cancellation failed")
	}
}
func TestTrumpTurnPassAndUsedCards(t *testing.T) {
	st := fixture()
	st = enterTrumpFight(st)
	if st.FightTurnSeat != 1 {
		t.Fatal("fight must start after bottom owner")
	}
	if _, err := Reduce(st, "2", EvCallPass, nil); err == nil {
		t.Fatal("out-of-turn pass accepted")
	}
	for _, uid := range []string{"1", "2", "3"} {
		r, err := Reduce(st, uid, EvCallPass, nil)
		if err != nil {
			t.Fatal(err)
		}
		st = r.State
	}
	if st.Phase != PhasePlayTrick {
		t.Fatal("passes did not start play")
	}
	st = fixture()
	if err := useTrumpCards(&st, []int{52, 1}); err != nil {
		t.Fatal(err)
	}
	if err := useTrumpCards(&st, []int{52, 2}); err == nil {
		t.Fatal("reused trump card accepted")
	}
	st.Phase = PhaseCallTrump
	st.CallMode = CallModeRace
	st.CallerSeat = -1
	st.CallPassMask = 1
	if _, err := Reduce(st, "0", EvCallTrump, CallTrumpPayload{53, []int{12}}); err == nil {
		t.Fatal("pass allowed reversal")
	}
}
func TestThrowUsesGlobalWeakestAndIncludesPartner(t *testing.T) {
	st := fixture()
	sc := rules.SCS
	card := func(id, value int) rules.Card {
		return rules.Card{ID: id, Suit: rules.Spade, Rank: rules.Rank(fmt.Sprint(value)), SuitClass: sc}
	}
	low := card(1, 3)
	high := card(2, 7)
	st.Seats[1].Hand = []rules.Card{card(3, 8)}
	st.Seats[2].Hand = []rules.Card{card(4, 5)}
	blocks, _ := rules.DecomposeThrow([]rules.Card{high, low}, st.Trump.Trump, sc)
	ok, move, _, err := canonicalizeLead(&st, 0, sc, Move{Blocks: blocks, Cards: []rules.Card{high, low}})
	if err != nil || ok || len(move.Cards) != 1 || move.Cards[0].ID != low.ID {
		t.Fatalf("wrong forced move %+v %v", move, err)
	}
	st.Seats[1].Hand = nil
	ok, _, _, _ = canonicalizeLead(&st, 0, sc, Move{Blocks: blocks})
	if ok {
		t.Fatal("partner must block throw")
	}
}
func TestMixedBottomMultiplierAndAttack(t *testing.T) {
	st := fixture()
	st.Bottom = []rules.Card{{Rank: rules.R10}}
	st.Trick.LastPlays[1] = &PlayedMove{Move: Move{Blocks: [][]rules.Block{{{Type: rules.BlockSingle}}, {{Type: rules.BlockTractor}}}}}
	settleDigBottom(&st, 1)
	if st.BottomMul != 4 || st.Points != 40 {
		t.Fatal("mixed move must use highest multiplier")
	}
	for _, owner := range []int{0, 1} {
		st = fixture()
		st.Trump.HasTrumpSuit = false
		st.BottomOwnerSeat = owner
		ok, _ := canDigBottom(&st, 1)
		if ok != (owner == 0) {
			t.Fatal("attack dig eligibility wrong")
		}
	}
}
func TestRoundResetAndPrivateView(t *testing.T) {
	st := fixture()
	st.Phase = PhaseRoundSettle
	st.NextStarterSeat = 0
	st.BottomPoints = 40
	st.BottomMul = 4
	st.BottomAward = 160
	st.BottomReveal = []rules.Card{{ID: 1}}
	st.SurrenderMask = 1
	st.UsedTrumpIDs[1] = true
	r, err := Reduce(st, "0", EvStartNextRound, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Notice, "第2局") {
		t.Fatalf("incorrect displayed round: %s", r.Notice)
	}
	st = r.State
	if st.BottomPoints != 0 || st.BottomMul != 0 || st.BottomAward != 0 || len(st.BottomReveal) != 0 || st.SurrenderMask != 0 || len(st.UsedTrumpIDs) != 0 {
		t.Fatal("round fields leaked")
	}
	v := MakeView(st, "spectator")
	if len(v.MyHand) != 0 || len(v.MyBottom) != 0 || len(v.UsedTrumpIDs) != 0 {
		t.Fatal("private cards leaked")
	}
}
func TestErrorsAreImmutable(t *testing.T) {
	base := NewErr("TEST", "base")
	a := base.WithInfo("a")
	b := base.WithInfof("%s", "b")
	if base.Info != "" || a.Info != "a" || b.Info != "b" {
		t.Fatal("shared errors mutated")
	}
}
