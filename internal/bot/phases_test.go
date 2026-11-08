package bot

import (
	"testing"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func phaseState() game.ViewState {
	s := game.ViewState{MySeat: 0, BottomOwnerSeat: 1, Phase: game.PhaseCallTrump}
	s.Teams[0].LevelRank = rules.R2
	s.Seats[1].Team = 1
	s.Trump.LevelRank = rules.R2
	s.Trump.CallerSeat = 1
	h := []rules.Card{}
	for _, c := range rules.NewDoubleDeck() {
		if c.Suit == rules.Heart || c.Suit == rules.BigJoker {
			h = append(h, c)
		}
	}
	s.MyHand = [][]rules.Card{h}
	return s
}
func TestCallAndUsedCards(t *testing.T) {
	s := phaseState()
	a, err := phaseAction(s)
	if err != nil || a.Type != "game.call_trump" {
		t.Fatalf("%+v %v", a, err)
	}
	p := a.Payload.(game.CallTrumpPayload)
	if len(p.LevelIDs) != 2 {
		t.Fatal("should lock strong suit")
	}
	s.UsedTrumpIDs = []int{53, 107}
	a, err = phaseAction(s)
	if err != nil || a.Type != "game.call_pass" {
		t.Fatalf("used jokers must not be reused: %+v %v", a, err)
	}
}
func TestChangeAndLocked(t *testing.T) {
	s := phaseState()
	s.Phase = game.PhaseTrumpFight
	s.Trump.HasTrumpSuit = true
	s.Trump.Suit = rules.Spade
	a, err := phaseAction(s)
	if err != nil || a.Type != "game.change_trump" {
		t.Fatalf("%+v %v", a, err)
	}
	s.Trump.Locked = true
	a, err = phaseAction(s)
	if err != nil || a.Type == "game.change_trump" {
		t.Fatalf("locked: %+v %v", a, err)
	}
}
func TestAttackAndPartnerRestraint(t *testing.T) {
	s := phaseState()
	s.Phase = game.PhaseTrumpFight
	s.Trump.HasTrumpSuit = true
	s.Trump.Suit = rules.Spade
	s.Trump.Locked = true
	s.MyHand = [][]rules.Card{{{ID: 53, Suit: rules.BigJoker, Rank: rules.RBJ}, {ID: 107, Suit: rules.BigJoker, Rank: rules.RBJ}}}
	a, err := phaseAction(s)
	if err != nil || a.Type != "game.attack_trump" {
		t.Fatalf("%+v %v", a, err)
	}
	s.Seats[1].Team = 0
	a, err = phaseAction(s)
	if err != nil || a.Type != "game.call_pass" {
		t.Fatalf("partner: %+v %v", a, err)
	}
}
func TestBottomLegalAndPreservesTrump(t *testing.T) {
	h := rules.NewDoubleDeck()[:33]
	tr := rules.Trump{HasTrumpSuit: true, Suit: rules.Spade, LevelRank: rules.R2}
	ids, err := bottom(h, tr)
	if err != nil || len(ids) != 8 {
		t.Fatalf("%v %v", ids, err)
	}
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatal("duplicate discard")
		}
		seen[id] = true
		found := false
		for _, c := range h {
			if c.ID == id {
				found = true
				if rules.ComputeSuitClass(c, tr) == rules.SCTrump {
					t.Fatal("discarded trump despite enough side cards")
				}
			}
		}
		if !found {
			t.Fatal("not in hand")
		}
	}
	if _, err := bottom(h[:25], tr); err == nil {
		t.Fatal("invalid bottom hand accepted")
	}
}
