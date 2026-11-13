package bot

import (
	"slices"
	"testing"
)

func TestLeafTrainingUsesRootTeamAndLeavesActorWeightsAlone(t *testing.T) {
	m := NewLearnModel(84)
	original := slices.Clone(m.Weights)
	var state learnState
	state[0] = .5
	row := []LeafTarget{{state: state, sign: -1, Levels: 1}, {state: state, sign: 1, Levels: -1}, {Levels: 0}}
	_, before := row[0].prediction(m)
	for range 60 {
		m.FitLeafBatch([][]LeafTarget{row}, .002)
	}
	_, after := row[0].prediction(m)
	_, opposite := row[1].prediction(m)
	_, settled := row[2].prediction(m)
	if after <= before || after != -opposite || settled != 0 || m.PickLeaf(row) != 0 {
		t.Fatalf("before %g after %g opposite %g settled %g", before, after, opposite, settled)
	}
	n := m.shape()
	if !slices.Equal(original[n.actionWeights:n.valueHead], m.Weights[n.actionWeights:n.valueHead]) || !slices.Equal(original[n.policySkip:], m.Weights[n.policySkip:]) {
		t.Fatal("action-specific weights changed")
	}
}
