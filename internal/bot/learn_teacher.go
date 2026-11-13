package bot

import (
	"fmt"
	"math"

	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
)

// ExactSample caches candidate features and signed round values at the root.
// Unlike an on-policy sample it stays valid as the actor's weights change.
type ExactSample struct {
	input                                    LearnSample
	values                                   []int
	bestLegal, bestCandidate, worstCandidate int
}

func (s ExactSample) Covered() bool     { return s.bestLegal == s.bestCandidate }
func (s ExactSample) Informative() bool { return s.bestCandidate > s.worstCandidate }

// Canonical entity sets allow arbitrary order without merging differing throws.
func teacherKey(ids []int) ([2]uint64, error) {
	var key [2]uint64
	if len(ids) == 0 {
		return key, fmt.Errorf("empty teacher action")
	}
	for _, id := range ids {
		if id < 0 || id >= 108 {
			return key, fmt.Errorf("invalid teacher card %d", id)
		}
		mask := uint64(1) << (id % 64)
		if key[id/64]&mask != 0 {
			return key, fmt.Errorf("duplicated card %d", id)
		}
		key[id/64] |= mask
	}
	return key, nil
}

func (m *LearnModel) LabelExact(table Table, choices []endgame.Choice) (ExactSample, error) {
	var s ExactSample
	if len(choices) == 0 {
		return s, fmt.Errorf("teacher returned no choices")
	}
	moves, input, _, err := m.choices(table)
	if err != nil {
		return s, err
	}
	s.input = input
	s.values = make([]int, len(moves))
	s.bestLegal = choices[0].NetLevels
	label := make(map[[2]uint64]int, len(choices))
	for _, c := range choices {
		key, err := teacherKey(c.Cards.CardIDs)
		if err != nil {
			return s, err
		}
		if prior, ok := label[key]; ok && prior != c.NetLevels {
			return s, fmt.Errorf("conflicting teacher labels for %v", c.Cards.CardIDs)
		}
		label[key] = c.NetLevels
		s.bestLegal = max(s.bestLegal, c.NetLevels)
	}
	for i, mv := range moves {
		ids := learnedAction(mv).Payload.(game.PlayCardsPayload).CardIDs
		key, err := teacherKey(ids)
		if err != nil {
			return s, err
		}
		value, ok := label[key]
		if !ok {
			return s, fmt.Errorf("missing exact label for %v", ids)
		}
		s.values[i] = value
		if i == 0 {
			s.bestCandidate, s.worstCandidate = value, value
		}
		s.bestCandidate = max(s.bestCandidate, value)
		s.worstCandidate = min(s.worstCandidate, value)
	}
	return s, nil
}

func (m *LearnModel) exactScores(s *ExactSample) ([]float64, []hiddenLearn, error) {
	n := m.shape()
	if s.input.variant != m.Variant || len(s.values) == 0 || len(s.values) != s.input.count || len(s.input.moves) != len(s.values)*n.moveSize {
		return nil, nil, fmt.Errorf("incompatible teacher sample")
	}
	h, _ := m.embed(&s.input.state)
	logits := make([]float64, len(s.values))
	acts := make([]hiddenLearn, len(logits))
	for i := range logits {
		logits[i], acts[i] = m.logit(&h, s.input.moves[i*n.moveSize:(i+1)*n.moveSize])
	}
	return logits, acts, nil
}

func (m *LearnModel) ExactRegret(s ExactSample) (int, error) {
	logits, _, err := m.exactScores(&s)
	if err != nil {
		return 0, err
	}
	best := 0
	for i := range logits {
		if logits[i] > logits[best] {
			best = i
		}
	}
	return s.bestLegal - s.values[best], nil
}

// teacherGradient accumulates the gradient of log likelihood, with a uniform
// target on all optimal candidate intents. Use log-sum-exp for stable loss.
func (m *LearnModel) teacherGradient(s *ExactSample, grad []float64) (float64, error) {
	logits, acts, err := m.exactScores(s)
	if err != nil {
		return 0, err
	}
	n := m.shape()
	peak := logits[0]
	optimal := 0
	for i, v := range logits {
		peak = max(peak, v)
		if s.values[i] == s.bestCandidate {
			optimal++
		}
	}
	var expSum, loss float64
	for i, v := range logits {
		expSum += math.Exp(v - peak)
		if s.values[i] == s.bestCandidate {
			loss += (peak - v) / float64(optimal)
		}
	}
	loss += math.Log(expSum)
	for i, v := range logits {
		delta := -math.Exp(v-peak) / expSum
		if s.values[i] == s.bestCandidate {
			delta += 1 / float64(optimal)
		}
		row := s.input.moves[i*n.moveSize : (i+1)*n.moveSize]
		for k, x := range row {
			grad[n.policySkip+k] += delta * x
		}
		for j, a := range acts[i][:n.width] {
			grad[n.policyHead+j] += delta * a
			d := delta * m.Weights[n.policyHead+j] * (1 - a*a)
			grad[n.actionBias+j] += d
			g := grad[n.actionWeights+j*n.moveSize : n.actionWeights+(j+1)*n.moveSize]
			for k, x := range row {
				g[k] += d * x
			}
		}
	}
	return loss, nil
}

// UpdateExact fits the policy head, action encoder and action skip connection.
// No shared state-encoder or value-head parameters receive an optimizer step.
func (m *LearnModel) UpdateExact(samples []ExactSample, rate float64) (float64, error) {
	if len(samples) == 0 || !finiteLearn(rate) || rate <= 0 || rate > 1 {
		return 0, fmt.Errorf("invalid teacher batch or rate")
	}
	n := m.shape()
	grad := make([]float64, n.parameters)
	var total float64
	for i := range samples {
		loss, err := m.teacherGradient(&samples[i], grad)
		if err != nil {
			return 0, fmt.Errorf("teacher sample %d: %w", i, err)
		}
		total += loss
	}
	if m.first == nil {
		m.first = make([]float64, n.parameters)
		m.second = make([]float64, n.parameters)
	}
	m.steps++
	firstCorrection := 1 - math.Pow(.9, float64(m.steps))
	secondCorrection := 1 - math.Pow(.999, float64(m.steps))
	for i := n.actionWeights; i < n.parameters; i++ {
		if i >= n.valueHead && i < n.policySkip {
			continue
		}
		g := grad[i] / float64(len(samples))
		m.first[i] = .9*m.first[i] + .1*g
		m.second[i] = .999*m.second[i] + .001*g*g
		m.Weights[i] += rate * (m.first[i] / firstCorrection) / (math.Sqrt(m.second[i]/secondCorrection) + 1e-8)
	}
	return total / float64(len(samples)), nil
}
