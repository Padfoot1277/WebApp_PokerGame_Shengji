package bot

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestLearnPolicyGradientAndRoundTrip(t *testing.T) {
	h := []rules.Card{realCard(rules.Club, rules.RA, 0), realCard(rules.Heart, rules.R5, 0), realCard(rules.Spade, rules.R9, 0)}
	st := fullFixture([4][]rules.Card{h, nil, nil, nil})
	table := Observe(st, 0)
	model := NewLearnModel(17)
	_, sample, err := model.Play(table, rand.New(rand.NewSource(9)))
	if err != nil {
		t.Fatal(err)
	}
	before, err := model.Value(table)
	if err != nil {
		t.Fatal(err)
	}
	_, _, probs, err := model.choices(table)
	if err != nil {
		t.Fatal(err)
	}
	sample.Reward = 1
	if err := model.Update([]LearnSample{sample}, .001); err != nil {
		t.Fatal(err)
	}
	after, _ := model.Value(table)
	_, _, changed, _ := model.choices(table)
	if changed[sample.chosen] <= probs[sample.chosen] || after <= before {
		t.Fatalf("positive reward did not increase action probability/value: %v %v; %v %v", probs, changed, before, after)
	}
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var loaded LearnModel
	if err = json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if err = loaded.Validate(); err != nil {
		t.Fatal(err)
	}
	orig, _ := model.Rank(table, 3)
	copy, _ := loaded.Rank(table, 3)
	if !reflect.DeepEqual(orig, copy) {
		t.Fatal("snapshot changed policy")
	}
	loaded.Weights[0] = math.NaN()
	if loaded.Validate() == nil {
		t.Fatal("invalid model accepted")
	}
}

func TestLearnCandidatesCompleteSeededRound(t *testing.T) {
	rng := rand.New(rand.NewSource(68))
	deck := rules.NewDoubleDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	h, b := rules.Deal(deck)
	st := fullFixture(h)
	st.Bottom = b
	model := NewLearnModel(33)
	for turns := 0; st.Phase == game.PhasePlayTrick; turns++ {
		if turns >= 100 {
			t.Fatal("round did not terminate")
		}
		s := st.Trick.TurnSeat
		table := Observe(st, s)
		a, _, err := model.Play(table, rng)
		if err != nil {
			t.Fatal(err)
		}
		if turns%9 == 0 {
			moves, err := learnCandidates(table)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range moves {
				// Clone before each branch because the reducer owns its slices.
				testState := st
				for seat := range st.Seats {
					testState.Seats[seat].Hand = append([]rules.Card(nil), st.Seats[seat].Hand...)
				}
				if _, e := game.Reduce(testState, st.Seats[s].UID, game.EvPlayCards, learnedAction(c).Payload); e != nil {
					t.Fatalf("candidate rejected at turn %d: %v", turns, e)
				}
			}
		}
		res, e := game.Reduce(st, st.Seats[s].UID, game.EvPlayCards, a.Payload)
		if e != nil {
			t.Fatal(e)
		}
		st = res.State
	}
}
