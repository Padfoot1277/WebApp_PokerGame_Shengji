package bot

import "math"

// LeafTarget stores the root team's terminal return and the next leader's
// perspective. A zero perspective denotes an already settled branch.
type LeafTarget struct {
	state  learnState
	sign   float64
	Levels int
}

func NextTrickTarget(table Table, sameTeam bool, levels int) LeafTarget {
	sign := -1.0
	if sameTeam {
		sign = 1
	}
	return LeafTarget{state: encodeLearnState(table), sign: sign, Levels: levels}
}

func (x *LeafTarget) prediction(m *LearnModel) (hiddenLearn, float64) {
	if x.sign == 0 {
		return hiddenLearn{}, float64(x.Levels) / 3
	}
	h, v := m.embed(&x.state)
	return h, x.sign * v
}

func (m *LearnModel) PickLeaf(row []LeafTarget) int {
	best := 0
	high := math.Inf(-1)
	for i := range row {
		_, score := row[i].prediction(m)
		if score > high {
			high, best = score, i
		}
	}
	return best
}

// FitLeafBatch regresses differences in candidate returns from the same root.
// The absolute-return term anchors the scale used beside exact terminal leaves.
// The caller trains a separate copy so its encoder updates never alter ranking
// or simulated replies in the deployed search path.
func (m *LearnModel) FitLeafBatch(rows [][]LeafTarget, rate float64) {
	n := m.shape()
	grad := make([]float64, n.parameters)
	for _, row := range rows {
		h := make([]hiddenLearn, len(row))
		scores := make([]float64, len(row))
		errors := make([]float64, len(row))
		mean := 0.0
		for i := range row {
			h[i], scores[i] = row[i].prediction(m)
			errors[i] = float64(row[i].Levels)/3 - scores[i]
			mean += errors[i] / float64(len(row))
		}
		for i, leaf := range row {
			if leaf.sign == 0 {
				continue
			}
			// Centered squared error equals a scaled sum of pairwise squared errors.
			d := (errors[i] - mean + 0.25*errors[i]) * leaf.sign * (1 - scores[i]*scores[i]) / float64(len(row))
			grad[n.valueBias] += d
			for j := range n.width {
				v := h[i][j]
				grad[n.valueHead+j] += d * v
				dh := d * m.Weights[n.valueHead+j] * (1 - v*v)
				grad[n.stateBias+j] += dh
				for k, x := range leaf.state {
					grad[j*learnStateSize+k] += dh * x
				}
			}
		}
	}
	if m.first == nil {
		m.first = make([]float64, n.parameters)
		m.second = make([]float64, n.parameters)
	}
	m.steps++
	a, b := 1-math.Pow(.9, float64(m.steps)), 1-math.Pow(.999, float64(m.steps))
	for i, g := range grad {
		if i >= n.actionWeights && (i < n.valueHead || i > n.valueBias) {
			continue
		}
		g /= float64(len(rows))
		m.first[i] = .9*m.first[i] + .1*g
		m.second[i] = .999*m.second[i] + .001*g*g
		m.Weights[i] += rate * (m.first[i] / a) / (math.Sqrt(m.second[i]/b) + 1e-8)
	}
}
