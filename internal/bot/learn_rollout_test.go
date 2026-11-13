package bot

import (
	"math"
	"testing"

	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestRolloutsTrainWithinShortlist(t *testing.T) {
	h := []rules.Card{realCard(rules.Club, rules.RA, 0), realCard(rules.Heart, rules.R5, 0), realCard(rules.Spade, rules.R9, 0)}
	table := Observe(fullFixture([4][]rules.Card{h, nil, nil, nil}), 0)
	model := NewLearnModel(139)
	options := make([]endgame.Choice, 2)
	for i := range options {
		options[i] = endgame.Choice{Cards: game.PlayCardsPayload{CardIDs: []int{h[i].ID}}, NetLevels: i}
	}
	sample, err := model.LabelRollouts(table, options)
	if err != nil || sample.input.count != 2 || !sample.Informative() {
		t.Fatalf("shortlist %v: %v", sample.values, err)
	}
	logits, _, err := model.exactScores(&sample)
	if err != nil {
		t.Fatal(err)
	}
	normalizeLearn(logits)
	for i, v := range sample.values {
		if v != 1 {
			continue
		}
		loss, err := model.UpdateExact([]ExactSample{sample}, .001)
		if err != nil || math.Abs(loss+math.Log(logits[i])) > 1e-10 {
			t.Fatalf("conditional loss %g expected %g, err %v", loss, -math.Log(logits[i]), err)
		}
	}
	if _, err = model.LabelRollouts(table, append(options, options[0])); err == nil {
		t.Fatal("duplicate branch accepted")
	}
	options[0].Cards.CardIDs = []int{108}
	if _, err = model.LabelRollouts(table, options); err == nil {
		t.Fatal("invalid branch accepted")
	}
}
