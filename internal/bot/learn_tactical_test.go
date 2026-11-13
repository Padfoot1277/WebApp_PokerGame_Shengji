package bot

import (
	"reflect"
	"testing"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func tacticalFixture() game.GameState {
	card := realCard
	return fullFixture([4][]rules.Card{
		{card(rules.Heart, rules.R7, 0), card(rules.Heart, rules.R7, 1), card(rules.Club, rules.RK, 0)},
		{card(rules.Spade, rules.R4, 0), card(rules.Spade, rules.R4, 1), card(rules.Club, rules.RQ, 0)},
		{card(rules.Heart, rules.RA, 0), card(rules.Heart, rules.RA, 1), card(rules.Club, rules.RJ, 0)},
		{card(rules.Spade, rules.RK, 0), card(rules.Club, rules.R8, 0), card(rules.Club, rules.R9, 0)},
	})
}

func TestTacticalRepliesAndLeadTransfer(t *testing.T) {
	st := tacticalFixture()
	table := Observe(st, 0)
	ctx := prepareTactics(table)
	f := make([]float64, tacticalExtraSize)
	ctx.encode(&table, table.Hands[0][:2], f)
	// Self leads a pair. Next seat can ruff with a pair, partner can
	// beat this incumbent in suit, last seat has just one trump.
	if f[1] != 1 || f[4] != 1 || f[8] != 1 || f[9] != 1 || f[10] != 0 {
		t.Fatalf("lead pair features: %v", f)
	}
	first, err := game.Reduce(st, "0", game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{st.Seats[0].Hand[0].ID, st.Seats[0].Hand[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	table = Observe(first.State, 1)
	ctx = prepareTactics(table)
	clear(f)
	ctx.encode(&table, table.Hands[1][:2], f)
	// After the ruff, the higher in-suit pair cannot retake. The already
	// played leader is not a remaining threat, and the new incumbent is self.
	if f[4] != 1 || f[8] != 0 || f[9] != 0 || f[10] != 0 {
		t.Fatalf("after ruff: %v", f)
	}
	// Give next seat one heart. Even with its spade pair a partial follow
	// is compulsory; a two-card ruff is now illegal.
	st.Seats[1].Hand[2] = classified([]rules.Card{realCard(rules.Heart, rules.R8, 0)}, st.Trump.Trump)[0]
	table = Observe(st, 0)
	ctx = prepareTactics(table)
	clear(f)
	ctx.encode(&table, table.Hands[0][:2], f)
	if f[8] != 0 || f[11] != 0 {
		t.Fatal("forced mixed response incorrectly considered a winning ruff")
	}
}

func TestTacticalExpansionAndMask(t *testing.T) {
	base := NewLearnModel(472)
	for i := range base.Weights {
		base.Weights[i] += .012
	}
	model, err := ExpandTactical(base)
	if err != nil {
		t.Fatal(err)
	}
	table := Observe(tacticalFixture(), 0)
	before, err := base.Rank(table, 32)
	if err != nil {
		t.Fatal(err)
	}
	after, err := model.Rank(table, 32)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("expanded network changed scores: %v", err)
	}
	for i := range model.Weights {
		model.Weights[i] += .027
	}
	mask, err := model.WithoutTacticalInputs()
	if err != nil {
		t.Fatal(err)
	}
	// Clear only the newly added columns directly in a copy; compare that
	// network's outputs to the independently constructed compact checkpoint.
	n := model.shape()
	for j := range n.width {
		off := n.actionWeights + j*n.moveSize
		clear(model.Weights[off+learnMoveSize : off+n.moveSize])
	}
	clear(model.Weights[n.policySkip+learnMoveSize:])
	exp, err := model.Rank(table, 32)
	if err != nil {
		t.Fatal(err)
	}
	got, err := mask.Rank(table, 32)
	if err != nil || !reflect.DeepEqual(got, exp) {
		t.Fatalf("ablation changed common weights: %v", err)
	}
	v, err := mask.Value(table)
	want, _ := model.Value(table)
	if err != nil || v != want {
		t.Fatalf("critic changed on ablation: %v %v", v, want)
	}
}

func TestTacticalPartnerStaysOnLead(t *testing.T) {
	st := tacticalFixture()
	// Seat 0 leads a king; seat 1 follows low. Seat 2 can duck under
	// partner's winner, so the incumbent relative to seat 2 is offset 2.
	for _, rank := range []rules.Rank{rules.RK, rules.RQ} {
		seat := st.Trick.TurnSeat
		card := realCard(rules.Club, rank, 0)
		result, err := game.Reduce(st, st.Seats[seat].UID, game.EvPlayCards, game.PlayCardsPayload{CardIDs: []int{card.ID}})
		if err != nil {
			t.Fatal(err)
		}
		st = result.State
	}
	table := Observe(st, 2)
	f := make([]float64, tacticalExtraSize)
	ctx := prepareTactics(table)
	ctx.encode(&table, table.Hands[2][2:], f)
	if f[6] != 1 || f[4] != 0 || f[0] != 1 || f[8] != 0 {
		t.Fatalf("partner control lost: %v", f)
	}
}

func BenchmarkRankTactical(b *testing.B) {
	hands, bottom := rules.Deal(rules.NewDoubleDeck())
	st := fullFixture(hands)
	st.Bottom = bottom
	table := Observe(st, 0)
	for _, name := range []string{"baseline", "tactical"} {
		m, err := NewLearnVariant(12, name)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				if _, err := m.Rank(table, 4); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
