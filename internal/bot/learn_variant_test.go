package bot

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	"upgrade-lan/internal/game/rules"
)

func TestVariantsPreserveInitialFunctionAndTrain(t *testing.T) {
	h := []rules.Card{realCard(rules.Club, rules.RA, 0), realCard(rules.Heart, rules.R5, 0), realCard(rules.Spade, rules.R9, 0)}
	table := Observe(fullFixture([4][]rules.Card{h}), 0)
	base := NewLearnModel(731)
	want, err := base.Rank(table, 5)
	if err != nil {
		t.Fatal(err)
	}
	baseV, err := base.Value(table)
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"wide", "rich"} {
		t.Run(variant, func(t *testing.T) {
			model, err := NewLearnVariant(731, variant)
			if err != nil {
				t.Fatal(err)
			}
			got, err := model.Rank(table, 5)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("initial ranking %v, expected %v, err %v", got, want, err)
			}
			v, err := model.Value(table)
			if err != nil || v != baseV {
				t.Fatalf("initial value %v, expected %v", v, baseV)
			}
			_, sample, err := model.Play(table, rand.New(rand.NewSource(7)))
			if err != nil {
				t.Fatal(err)
			}
			sample.Reward = 1
			if err := model.Update([]LearnSample{sample}, .001); err != nil {
				t.Fatal(err)
			}
			n := model.shape()
			extra := model.Weights[n.policyHead+16 : n.valueHead]
			if variant == "rich" {
				extra = model.Weights[n.policySkip+learnMoveSize:]
			}
			changed := false
			for _, w := range extra {
				if w != 0 {
					changed = true
					break
				}
			}
			if !changed {
				t.Fatal("extra weights did not learn")
			}
			raw, err := json.Marshal(model)
			if err != nil {
				t.Fatal(err)
			}
			var restored LearnModel
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			if err := restored.Validate(); err != nil {
				t.Fatal(err)
			}
			before, err := model.Rank(table, 5)
			if err != nil {
				t.Fatal(err)
			}
			after, err := restored.Rank(table, 5)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("rank mismatch after checkpoint round trip")
			}
			restored.Version = 1
			if restored.Validate() == nil {
				t.Fatal("incompatible architecture accepted as legacy")
			}
		})
	}
}
