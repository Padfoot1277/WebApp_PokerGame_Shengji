package bot

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// A state embedding is reused across all candidate actions. The policy combines
// it with a learned action embedding; the critic predicts the terminal signed
// team return from the state alone.
const maxLearnWidth = 64

type networkShape struct {
	width, moveSize                                                                                int
	stateBias, actionWeights, actionBias, policyHead, valueHead, valueBias, policySkip, parameters int
}

func shape(width, moves int) networkShape {
	n := networkShape{width: width, moveSize: moves}
	n.stateBias = width * learnStateSize
	n.actionWeights = n.stateBias + width
	n.actionBias = n.actionWeights + width*moves
	n.policyHead = n.actionBias + width
	n.valueHead = n.policyHead + width
	n.valueBias = n.valueHead + width
	n.policySkip = n.valueBias + 1
	n.parameters = n.policySkip + moves
	return n
}

var baselineShape = shape(16, learnMoveSize)
var wideShape = shape(maxLearnWidth, learnMoveSize)
var richShape = shape(16, learnMoveSize+richExtraSize)
var tacticalShape = shape(16, learnMoveSize+tacticalExtraSize)

type LearnModel struct {
	Version int       `json:"version"`
	Variant string    `json:"variant,omitempty"`
	Weights []float64 `json:"weights"`
	// Adam's moments are local to a training session. Inference files need only weights.
	first, second []float64
	steps         int
}
type LearnSample struct {
	state   learnState
	moves   []float64 // contiguous action rows
	count   int
	variant string
	chosen  int
	Reward  float64
}
type RankedPlay struct {
	Action      Action
	Probability float64
}

func (m *LearnModel) shape() networkShape {
	switch m.Variant {
	case "wide":
		return wideShape
	case "rich":
		return richShape
	case "tactical":
		return tacticalShape
	default:
		return baselineShape
	}
}

func NewLearnModel(seed int64) *LearnModel {
	n := baselineShape
	rng := rand.New(rand.NewSource(seed))
	p := make([]float64, n.parameters)
	for i := 0; i < n.stateBias; i++ {
		p[i] = rng.NormFloat64() * .08
	}
	for i := n.actionWeights; i < n.actionBias; i++ {
		p[i] = rng.NormFloat64() * .16
	}
	for i := n.policyHead; i < n.valueBias; i++ {
		p[i] = rng.NormFloat64() * .18
	}
	// Starting tactical preferences are trainable weights, shared by the
	// initialization baseline and every training iteration.
	p[n.policySkip+2] = -.6
	p[n.policySkip+5] = -.6
	p[n.policySkip+9] = .6
	p[n.policySkip+11] = 1.2
	p[n.policySkip+12] = 1.8
	return &LearnModel{Version: 1, Weights: p}
}

// NewLearnVariant preserves the baseline function at initialization. Rich-only
// inputs start with zero weights. Additional wide neurons have random inputs
// and zero output weights, which receive a gradient on the first update.
func NewLearnVariant(seed int64, variant string) (*LearnModel, error) {
	if variant == "tactical" {
		return ExpandTactical(NewLearnModel(seed))
	}
	if variant != "baseline" && variant != "rich" && variant != "wide" {
		return nil, fmt.Errorf("unknown network variant %q", variant)
	}
	base := NewLearnModel(seed)
	if variant == "baseline" {
		return base, nil
	}
	m := &LearnModel{Version: 2, Variant: variant}
	n, old := m.shape(), baselineShape
	m.Weights = make([]float64, n.parameters)
	p, q := m.Weights, base.Weights
	copy(p[:n.stateBias], q[:old.stateBias])
	copy(p[n.policyHead:], q[old.policyHead:old.valueHead])
	copy(p[n.valueHead:], q[old.valueHead:old.valueBias])
	copy(p[n.policySkip:], q[old.policySkip:])
	for j := range old.width {
		copy(p[n.actionWeights+j*n.moveSize:], q[old.actionWeights+j*old.moveSize:old.actionWeights+(j+1)*old.moveSize])
	}
	rng := rand.New(rand.NewSource(seed ^ 0x72f31))
	for j := old.width; j < n.width; j++ {
		for i := range learnStateSize {
			p[j*learnStateSize+i] = rng.NormFloat64() * .08
		}
		for i := range n.moveSize {
			p[n.actionWeights+j*n.moveSize+i] = rng.NormFloat64() * .16
		}
	}
	return m, nil
}

func (m *LearnModel) Validate() error {
	if m == nil || !(m.Version == 1 && m.Variant == "" || m.Version == 2 && (m.Variant == "rich" || m.Variant == "wide" || m.Variant == "tactical")) || len(m.Weights) != m.shape().parameters {
		return fmt.Errorf("incompatible learned model version or weight count")
	}
	for _, w := range m.Weights {
		if !finiteLearn(w) || math.Abs(w) > 1e4 {
			return fmt.Errorf("invalid learned model weight")
		}
	}
	return nil
}
func finiteLearn(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

type hiddenLearn [maxLearnWidth]float64

func (m *LearnModel) embed(x *learnState) (h hiddenLearn, value float64) {
	n := m.shape()
	p := m.Weights
	value = p[n.valueBias]
	for j := range n.width {
		z := p[n.stateBias+j]
		row := p[j*learnStateSize : (j+1)*learnStateSize]
		for i, xi := range x {
			z += row[i] * xi
		}
		h[j] = math.Tanh(z)
		value += p[n.valueHead+j] * h[j]
	}
	return h, math.Tanh(value)
}
func (m *LearnModel) logit(h *hiddenLearn, a []float64) (float64, hiddenLearn) {
	n := m.shape()
	p := m.Weights
	z := 0.
	for i, x := range a {
		z += p[n.policySkip+i] * x
	}
	var act hiddenLearn
	for j := range n.width {
		v := h[j] + p[n.actionBias+j]
		row := p[n.actionWeights+j*n.moveSize : n.actionWeights+(j+1)*n.moveSize]
		for i, x := range a {
			v += row[i] * x
		}
		act[j] = math.Tanh(v)
		z += p[n.policyHead+j] * act[j]
	}
	return z, act
}
func normalizeLearn(logits []float64) {
	high := logits[0]
	for _, v := range logits[1:] {
		if v > high {
			high = v
		}
	}
	total := 0.
	for i, v := range logits {
		logits[i] = math.Exp(v - high)
		total += logits[i]
	}
	for i := range logits {
		logits[i] /= total
	}
}
func (m *LearnModel) choices(table Table) ([][]rules.Card, LearnSample, []float64, error) {
	moves, err := learnCandidates(table)
	if err != nil {
		return nil, LearnSample{}, nil, err
	}
	n := m.shape()
	sample := LearnSample{state: encodeLearnState(table), moves: make([]float64, len(moves)*n.moveSize), count: len(moves), variant: m.Variant}
	var rich richContext
	var tactics tacticalContext
	if m.Variant == "tactical" {
		tactics = prepareTactics(table)
	}
	if m.Variant == "rich" {
		rich = prepareRich(table)
	}
	probs := make([]float64, len(moves))
	h, _ := m.embed(&sample.state)
	for i, cards := range moves {
		row := sample.moves[i*n.moveSize : (i+1)*n.moveSize]
		base := encodeLearnMove(table, cards)
		copy(row, base[:])
		if m.Variant == "rich" {
			rich.encode(cards, row[learnMoveSize:])
		}
		if m.Variant == "tactical" {
			tactics.encode(&table, cards, row[learnMoveSize:])
		}
		probs[i], _ = m.logit(&h, row)
	}
	normalizeLearn(probs)
	return moves, sample, probs, nil
}
func learnedAction(cards []rules.Card) Action {
	ids := make([]int, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	return Action{string(game.EvPlayCards), game.PlayCardsPayload{CardIDs: ids}}
}
func (m *LearnModel) Play(table Table, rng *rand.Rand) (Action, LearnSample, error) {
	moves, sample, probs, err := m.choices(table)
	if err != nil {
		return Action{}, LearnSample{}, err
	}
	chosen := 0
	if rng == nil {
		for i, p := range probs {
			if p > probs[chosen] {
				chosen = i
			}
		}
	} else {
		target := rng.Float64()
		chosen = len(probs) - 1
		for i, p := range probs {
			target -= p
			if target <= 0 {
				chosen = i
				break
			}
		}
	}
	sample.chosen = chosen
	return learnedAction(moves[chosen]), sample, nil
}
func (m *LearnModel) Rank(table Table, k int) ([]RankedPlay, error) {
	if k < 1 {
		return nil, fmt.Errorf("positive rank count required")
	}
	moves, _, probs, err := m.choices(table)
	if err != nil {
		return nil, err
	}
	out := make([]RankedPlay, len(moves))
	for i, cards := range moves {
		out[i] = RankedPlay{learnedAction(cards), probs[i]}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Probability > out[j].Probability })
	return out[:min(k, len(out))], nil
}
func (m *LearnModel) Value(table Table) (float64, error) {
	v := table.View
	if v.MySeat < 0 || v.MySeat >= 4 || v.Trick.LeaderSeat < 0 || v.Trick.LeaderSeat >= 4 {
		return 0, fmt.Errorf("invalid learned value seat")
	}
	x := encodeLearnState(table)
	_, value := m.embed(&x)
	return value, nil
}

// Update takes one on-policy batch from the current weights. Policy gradients
// use the sampled action's log probability and an action-independent critic
// baseline. The value target is the same team's observed terminal return.
func (m *LearnModel) Update(samples []LearnSample, rate float64) error {
	if len(samples) == 0 || rate <= 0 || rate > 1 || !finiteLearn(rate) {
		return fmt.Errorf("invalid training batch or learning rate")
	}
	n := m.shape()
	grad := make([]float64, n.parameters)
	for i := range samples {
		s := &samples[i]
		if s.variant != m.Variant || len(s.moves) != s.count*n.moveSize || s.count == 0 || s.chosen < 0 || s.chosen >= s.count || !finiteLearn(s.Reward) || math.Abs(s.Reward) > 1.000001 {
			return fmt.Errorf("invalid sample %d", i)
		}
		h, value := m.embed(&s.state)
		advantage := s.Reward - value
		dv := advantage * (1 - value*value)
		grad[n.valueBias] += dv
		var dh hiddenLearn
		for j := range n.width {
			grad[n.valueHead+j] += dv * h[j]
			dh[j] = dv * m.Weights[n.valueHead+j]
		}
		if s.count > 1 {
			probs := make([]float64, s.count)
			acts := make([]hiddenLearn, s.count)
			for k := range s.count {
				probs[k], acts[k] = m.logit(&h, s.moves[k*n.moveSize:(k+1)*n.moveSize])
			}
			normalizeLearn(probs)
			for k := range s.count {
				dl := advantage * (flag(k == s.chosen) - probs[k])
				a := s.moves[k*n.moveSize : (k+1)*n.moveSize]
				for l, x := range a {
					grad[n.policySkip+l] += dl * x
				}
				for j, v := range acts[k][:n.width] {
					grad[n.policyHead+j] += dl * v
					da := dl * m.Weights[n.policyHead+j] * (1 - v*v)
					grad[n.actionBias+j] += da
					dh[j] += da
					row := grad[n.actionWeights+j*n.moveSize : n.actionWeights+(j+1)*n.moveSize]
					for l, x := range a {
						row[l] += da * x
					}
				}
			}
		}
		for j, v := range h[:n.width] {
			d := dh[j] * (1 - v*v)
			grad[n.stateBias+j] += d
			row := grad[j*learnStateSize : (j+1)*learnStateSize]
			for k, x := range s.state {
				row[k] += d * x
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
		g /= float64(len(samples))
		m.first[i] = .9*m.first[i] + .1*g
		m.second[i] = .999*m.second[i] + .001*g*g
		m.Weights[i] += rate * (m.first[i] / a) / (math.Sqrt(m.second[i]/b) + 1e-8)
	}
	return nil
}
