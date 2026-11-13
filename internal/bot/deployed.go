package bot

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"

	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// LoadLearnedPolicy reads and validates a checkpoint before any worker sees it.
func LoadLearnedPolicy(path string) (*LearnModel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var model LearnModel
	if err := json.Unmarshal(raw, &model); err != nil {
		return nil, err
	}
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &model, nil
}

// LearnedPlay is the common inference path for arena evaluation and room workers.
// Each root alternative is played through the current trick against greedy
// learned follow-ups. Late positions small enough to solve use exact team minimax.
func LearnedPlay(st game.GameState, model *LearnModel) (action Action, err error) {
	return selectPlay(st, model, model, model, false)
}

// SearchWithModels isolates shortlist ranking, simulated replies, and the
// leaf value for offline comparisons while retaining the normal search rules.
func SearchWithModels(st game.GameState, rank, reply, value *LearnModel) (Action, error) {
	return selectPlay(st, rank, reply, value, false)
}

// HybridPlay retains the full-information tactics and compares close choices
// using the learned continuation value at the start of the next trick.
func HybridPlay(st game.GameState, model *LearnModel) (Action, error) {
	return selectPlay(st, model, model, model, true)
}

func selectPlay(st game.GameState, model, reply, leaf *LearnModel, hybrid bool) (action Action, err error) {
	// A failed experimental policy must hand the room back to its built-in policy.
	defer func() {
		if recovered := recover(); recovered != nil {
			action = Action{}
			err = fmt.Errorf("learned play failed: %v", recovered)
		}
	}()
	if model == nil || reply == nil || leaf == nil || st.Phase != game.PhasePlayTrick || st.Trick.TurnSeat < 0 || st.Trick.TurnSeat > 3 {
		return Action{}, fmt.Errorf("expected a model and a seat to play")
	}
	if endgame.Eligible(st) {
		cards, _, err := endgame.Best(st)
		if err != nil {
			return Action{}, err
		}
		return Action{Type: string(game.EvPlayCards), Payload: cards}, nil
	}

	seat := st.Trick.TurnSeat
	table := Observe(st, seat)
	if hybrid {
		return hybridChoice(st, table, model)
	}
	candidates, err := model.Rank(table, 3)
	if err != nil {
		return Action{}, err
	}
	bestValue := math.Inf(-1)
	best := Action{}
	for _, candidate := range candidates {
		value, err := rolloutTrick(st, candidate.Action, reply, leaf)
		if err != nil {
			return Action{}, err
		}
		if value > bestValue {
			bestValue, best = value, candidate.Action
		}
	}
	if best.Type == "" {
		return Action{}, fmt.Errorf("no evaluated candidate for seat %d", seat)
	}
	return best, nil
}

type tacticalChoice struct {
	cards []rules.Card
	score float64
}

func hybridChoice(st game.GameState, table Table, model *LearnModel) (Action, error) {
	var options []tacticalChoice
	_, err := scoreFull(table, func(cards []rules.Card, score float64) {
		options = append(options, tacticalChoice{cards, score})
	})
	if err != nil {
		return Action{}, err
	}
	sort.SliceStable(options, func(i, j int) bool { return options[i].score > options[j].score })

	// A single five-point scoring card is worth at least 15 tactical units.
	// A critic prediction in [-1,1] can bridge a gap of at most 6 units, so
	// only nearby options need rollout. Deck copies share a strategic action.
	const valueWeight = 3
	const maxOptions = 4
	seen := make(map[[54]byte]bool, maxOptions)
	var shortlist []tacticalChoice
	for _, option := range options {
		if options[0].score-option.score >= 2*valueWeight {
			break
		}
		var faces [54]byte
		for _, card := range option.cards {
			faces[card.ID%54]++
		}
		if !seen[faces] {
			seen[faces] = true
			shortlist = append(shortlist, option)
			if len(shortlist) == maxOptions {
				break
			}
		}
	}
	best := learnedAction(shortlist[0].cards)
	if len(shortlist) == 1 {
		return best, nil
	}
	bestScore := math.Inf(-1)
	for _, option := range shortlist {
		move := learnedAction(option.cards)
		value, err := rolloutTrick(st, move, model, model)
		if err != nil {
			return Action{}, err
		}
		score := option.score + valueWeight*value
		if score > bestScore {
			bestScore, best = score, move
		}
	}
	return best, nil
}

func rolloutTrick(st game.GameState, action Action, reply, leaf *LearnModel) (float64, error) {
	team := st.Seats[st.Trick.TurnSeat].Team
	next, err := SearchLeaf(st, action, reply)
	if err != nil {
		return 0, err
	}
	if next.Phase != game.PhasePlayTrick {
		value := next.CallerDelta - next.DefenderDelta
		if next.Seats[next.CallerSeat].Team != team {
			value = -value
		}
		return float64(value) / 3, nil
	}
	value, err := leaf.Value(Observe(next, next.Trick.TurnSeat))
	if next.Seats[next.Trick.TurnSeat].Team != team {
		value = -value
	}
	return value, err
}

// SearchLeaf advances the proposed move and greedy replies up to the same
// next-trick boundary used both for inference and for value training.
func SearchLeaf(st game.GameState, action Action, reply *LearnModel) (game.GameState, error) {
	index := st.TrickIndex
	for ply := 0; ply < 4; ply++ {
		result, problem := game.Reduce(st, st.Seats[st.Trick.TurnSeat].UID, game.EvPlayCards, action.Payload)
		if problem != nil {
			return st, problem
		}
		st = result.State
		if st.Phase == game.PhaseGameOver || st.Phase == game.PhaseRoundSettle || st.TrickIndex != index {
			return st, nil
		}
		replies, err := reply.Rank(Observe(st, st.Trick.TurnSeat), 1)
		if err != nil {
			return st, err
		}
		if len(replies) == 0 {
			return st, fmt.Errorf("no rollout reply for seat %d", st.Trick.TurnSeat)
		}
		action = replies[0].Action
	}
	return st, fmt.Errorf("trick %d did not advance in four moves", index)
}
