package game

import (
	"testing"
	"upgrade-lan/internal/game/rules"
)

func TestCallChangeAttackScenario(t *testing.T) {
	st := fixture()
	st.Phase = PhaseCallTrump
	st.CallMode = CallModeRace
	st.CallerSeat = -1
	st.Trump.LevelRank = rules.RPending
	deck := rules.NewDoubleDeck()
	required := [][]int{{53, 25}, {107, 51, 105}, {52, 106}, {}}
	used := map[int]bool{}
	for seat, ids := range required {
		st.Seats[seat].Hand = nil
		for _, id := range ids {
			st.Seats[seat].Hand = append(st.Seats[seat].Hand, deck[id])
			used[id] = true
		}
	}
	remaining := []rules.Card{}
	for _, c := range deck {
		if !used[c.ID] {
			remaining = append(remaining, c)
		}
	}
	for seat := range st.Seats {
		for len(st.Seats[seat].Hand) < 25 {
			st.Seats[seat].Hand = append(st.Seats[seat].Hand, remaining[0])
			remaining = remaining[1:]
		}
		st.Seats[seat].HandCount = 25
	}
	st.Bottom = remaining
	st.BottomCount = 8
	apply := func(uid string, typ ClientEventType, payload any) {
		t.Helper()
		r, e := Reduce(st, uid, typ, payload)
		if e != nil {
			t.Fatalf("%s: %v", typ, e)
		}
		st = r.State
	}
	discard := func(uid string) { t.Helper(); ids := getIDs(st.Bottom); apply(uid, EvPutBottom, PutBottomPayload{ids}) }
	apply("0", EvCallTrump, CallTrumpPayload{53, []int{25}})
	discard("0")
	if st.FightTurnSeat != 1 {
		t.Fatal("incorrect fight start")
	}
	apply("1", EvChangeTrump, ChangeTrumpPayload{107, []int{51, 105}})
	if st.CallerSeat != 0 || st.Trump.Suit != rules.Diamond || st.BottomOwnerSeat != 1 {
		t.Fatal("change altered caller or bottom owner")
	}
	discard("1")
	apply("2", EvAttackTrump, AttackTrumpPayload{[]int{52, 106}})
	discard("2")
	if st.CallerSeat != 0 || st.Trick.TurnSeat != 0 || st.Trump.HasTrumpSuit || st.Phase != PhasePlayTrick {
		t.Fatal("attack altered original caller/leader")
	}
	if ok, _ := canDigBottom(&st, 1); !ok {
		t.Fatal("caller team attacked; defenders should be able to dig")
	}
	// Every live card remains unique after three rounds of taking and burying bottom.
	seen := map[int]bool{}
	for _, seat := range st.Seats {
		for _, c := range seat.Hand {
			if seen[c.ID] {
				t.Fatal("duplicate card")
			}
			seen[c.ID] = true
		}
	}
	for _, c := range st.Bottom {
		if seen[c.ID] {
			t.Fatal("bottom duplicated hand")
		}
		seen[c.ID] = true
	}
	if len(seen) != 108 {
		t.Fatal("cards lost")
	}
	for _, id := range []int{53, 25, 107, 51, 105, 52, 106} {
		if !st.UsedTrumpIDs[id] {
			t.Fatalf("used trump card %d forgotten", id)
		}
	}
}
func TestPairAndTractorHideRejections(t *testing.T) {
	deck := rules.NewDoubleDeck()
	trump := rules.Trump{HasTrumpSuit: true, Suit: rules.Heart, LevelRank: rules.R2}
	cards := func(ids ...int) []rules.Card {
		out := []rules.Card{}
		for _, id := range ids {
			c := deck[id]
			c.SuitClass = rules.ComputeSuitClass(c, trump)
			out = append(out, c)
		}
		return out
	}
	lead, _ := rules.DecomposeThrow(cards(0, 54), trump, rules.SCS)
	hand := cards(1, 55, 2, 3)
	if e := rules.ValidateHideCheck(hand, cards(2, 3), lead, trump); e == nil {
		t.Fatal("hidden pair accepted")
	}
	if e := rules.ValidateHideCheck(hand, cards(1, 55), lead, trump); e != nil {
		t.Fatal(e)
	}
	lead, _ = rules.DecomposeThrow(cards(0, 54, 1, 55), trump, rules.SCS)
	hand = cards(2, 56, 3, 57, 4, 5)
	if e := rules.ValidateHideCheck(hand, cards(2, 56, 4, 5), lead, trump); e == nil {
		t.Fatal("hidden tractor accepted")
	}
	if e := rules.ValidateHideCheck(hand, cards(2, 56, 3, 57), lead, trump); e != nil {
		t.Fatal(e)
	}
}
