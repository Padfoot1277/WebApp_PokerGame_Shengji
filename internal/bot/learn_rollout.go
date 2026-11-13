package bot

import (
	"fmt"

	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
)

// LabelRollouts constructs a conditional classification problem on just the
// sampled candidates. Its regret is relative to the best simulated branch.
func (m *LearnModel) LabelRollouts(table Table, choices []endgame.Choice) (ExactSample, error) {
	var result ExactSample
	if len(choices) == 0 {
		return result, fmt.Errorf("empty rollout shortlist")
	}
	moves, input, _, err := m.choices(table)
	if err != nil {
		return result, err
	}
	labels := make(map[[2]uint64]int, len(choices))
	for _, choice := range choices {
		key, err := teacherKey(choice.Cards.CardIDs)
		if err != nil {
			return result, err
		}
		if _, exists := labels[key]; exists {
			return result, fmt.Errorf("duplicate rollout action %v", choice.Cards.CardIDs)
		}
		labels[key] = choice.NetLevels
	}
	dst := input
	dst.count = 0
	dst.moves = nil
	width := m.shape().moveSize
	// Use the same candidate order as Play, including its greedy tie break.
	for i, cards := range moves {
		ids := learnedAction(cards).Payload.(game.PlayCardsPayload).CardIDs
		key, err := teacherKey(ids)
		if err != nil {
			return result, err
		}
		v, ok := labels[key]
		if !ok {
			continue
		}
		delete(labels, key)
		dst.moves = append(dst.moves, input.moves[i*width:(i+1)*width]...)
		if dst.count == 0 {
			result.bestCandidate, result.worstCandidate = v, v
		}
		dst.count++
		result.values = append(result.values, v)
		result.bestCandidate = max(v, result.bestCandidate)
		result.worstCandidate = min(v, result.worstCandidate)
	}
	if len(labels) > 0 {
		return ExactSample{}, fmt.Errorf("%d rollout candidates absent from policy", len(labels))
	}
	result.bestLegal = result.bestCandidate
	result.input = dst
	return result, nil
}
