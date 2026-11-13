package bot

import (
	"math"
	"slices"
	"testing"

	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestTeacherGradientAndCoverage(t *testing.T) {
	h := []rules.Card{realCard(rules.Club, rules.RA, 0), realCard(rules.Heart, rules.R5, 0), realCard(rules.Spade, rules.R9, 0)}
	table := Observe(fullFixture([4][]rules.Card{h, nil, nil, nil}), 0)
	m := NewLearnModel(11)
	choices := make([]endgame.Choice, 3)
	for i, c := range h {
		choices[i] = endgame.Choice{Cards: game.PlayCardsPayload{CardIDs: []int{c.ID}}, NetLevels: 1}
	}
	choices[1].NetLevels = -1
	s, err := m.LabelExact(table, choices)
	if err != nil || !s.Informative() || !s.Covered() {
		t.Fatalf("bad label: %v", err)
	}
	if _, err = m.LabelExact(table, choices[:2]); err == nil {
		t.Fatal("accepted incomplete labels")
	}
	n := m.shape()
	grad := make([]float64, n.parameters)
	before, err := m.teacherGradient(&s, grad)
	if err != nil {
		t.Fatal(err)
	}
	// Uniform targets for the two best moves, independent of their ordering.
	logits, _, _ := m.exactScores(&s)
	normalizeLearn(logits)
	want := 0.
	for i, v := range s.values {
		if v == 1 {
			want -= math.Log(logits[i]) / 2
		}
	}
	if math.Abs(before-want) > 1e-12 {
		t.Fatalf("loss %.12g, want %.12g", before, want)
	}
	scratch := make([]float64, n.parameters)
	for _, i := range []int{n.policySkip + 1, n.policyHead + 3, n.actionBias + 1, n.actionWeights + n.moveSize + 1} {
		orig := m.Weights[i]
		const eps = 1e-6
		m.Weights[i] = orig + eps
		up, _ := m.teacherGradient(&s, scratch)
		m.Weights[i] = orig - eps
		down, _ := m.teacherGradient(&s, scratch)
		m.Weights[i] = orig
		if math.Abs(grad[i]+(up-down)/(2*eps)) > 1e-7 {
			t.Fatalf("gradient %d: %.9g numerical %.9g", i, grad[i], (down-up)/(2*eps))
		}
	}
	orig := slices.Clone(m.Weights)
	returned, err := m.UpdateExact([]ExactSample{s, s}, .005)
	if err != nil || returned != before {
		t.Fatalf("batch loss=%v error=%v", returned, err)
	}
	if !slices.Equal(orig[:n.actionWeights], m.Weights[:n.actionWeights]) || !slices.Equal(orig[n.valueHead:n.policySkip], m.Weights[n.valueHead:n.policySkip]) {
		t.Fatal("teacher changed state encoder or value head")
	}
	after, _ := m.teacherGradient(&s, scratch)
	if after >= before {
		t.Fatalf("teacher loss %.8f -> %.8f", before, after)
	}
}
