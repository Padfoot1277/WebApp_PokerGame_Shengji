package bot

import (
	"fmt"
	"sort"

	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// ShortRulePosition is the window where the room worker needs a full state
// for multi-trick planning. Outside it the historical one-trick policy runs.
func ShortRulePosition(st game.GameState) bool {
	return withinRuleWindow(st, roomRuleSearch.MaxHand)
}

func withinRuleWindow(st game.GameState, maxHand int) bool {
	if st.Phase != game.PhasePlayTrick || st.Trick.TurnSeat < 0 || st.Trick.TurnSeat >= 4 {
		return false
	}
	for _, seat := range st.Seats {
		if len(seat.Hand) > maxHand {
			return false
		}
	}
	return true
}

func ruleStep(st game.GameState, cards game.PlayCardsPayload) (game.GameState, error) {
	res, problem := game.Reduce(st, st.Seats[st.Trick.TurnSeat].UID, game.EvPlayCards, cards)
	if problem != nil {
		return st, problem
	}
	return res.State, nil
}

// Continue with the frozen tactical policy at all seats. If exactTail is
// enabled, both teams play optimally in the small ending and only the
// net level result is available.
func ruleReturn(st game.GameState, first game.PlayCardsPayload, exactTail int) (ruleOutcome, error) {
	team := st.Seats[st.Trick.TurnSeat].Team
	next, err := ruleStep(st, first)
	for err == nil && next.Phase == game.PhasePlayTrick {
		if exactTail > 0 && withinRuleWindow(next, exactTail) {
			levels, err := endgame.Value(next, team)
			return ruleOutcome{levels: levels}, err
		}
		table := Observe(next, next.Trick.TurnSeat)
		cards, e := scoreFullBudget(table, nil, true)
		if e != nil {
			return ruleOutcome{}, e
		}
		next, err = ruleStep(next, learnedAction(cards).Payload.(game.PlayCardsPayload))
	}
	if err != nil {
		return ruleOutcome{}, err
	}
	if next.Phase != game.PhaseRoundSettle && next.Phase != game.PhaseGameOver {
		return ruleOutcome{}, fmt.Errorf("unexpected continuation phase %s", next.Phase)
	}
	delta := next.CallerDelta - next.DefenderDelta
	margin := -next.RoundPointsFinal
	if next.Seats[next.CallerSeat].Team != team {
		delta = -delta
		margin = -margin
	}
	return ruleOutcome{delta, margin}, nil
}

type ruleOutcome struct {
	levels, margin int
}

func (o ruleOutcome) better(best ruleOutcome, preferPoints bool) bool {
	return o.levels > best.levels || (preferPoints && o.levels == best.levels && o.margin > best.margin)
}

// RulePlay uses the room's configured rule search.
func RulePlay(st game.GameState) (Action, error) {
	return roomRuleSearch.Play(st)
}

// SixCardRulePlay retains the six-card, three-candidate reference policy.
func SixCardRulePlay(st game.GameState) (Action, error) {
	return (RuleSearch{MaxHand: 6, Candidates: 3}).Play(st)
}

// RuleSearch controls when to start settlement rollouts and how many distinct
// tactical candidates to compare. Exact endings use the full legal move set.
type RuleSearch struct {
	MaxHand      int
	Candidates   int
	PreferPoints bool
	Diverse      bool
	ExactTail    int // use minimax to finish simulated lines at this hand size
}

var roomRuleSearch = RuleSearch{MaxHand: 15, Candidates: 5, ExactTail: 3}

// RoomRuleSearch returns the built-in room configuration for startup diagnostics.
func RoomRuleSearch() RuleSearch { return roomRuleSearch }

func (search RuleSearch) Play(st game.GameState) (Action, error) {
	if search.MaxHand < endgame.MaxHand || search.Candidates < 1 || search.ExactTail < 0 || search.ExactTail > endgame.MaxHand {
		return Action{}, fmt.Errorf("invalid rule search limits")
	}
	if search.ExactTail > 0 && search.PreferPoints {
		return Action{}, fmt.Errorf("exact tails supply net levels only")
	}
	if st.Phase != game.PhasePlayTrick || st.Trick.TurnSeat < 0 || st.Trick.TurnSeat >= 4 {
		return Action{}, fmt.Errorf("expected active play seat")
	}
	table := Observe(st, st.Trick.TurnSeat)
	if !withinRuleWindow(st, search.MaxHand) {
		return DecideFull(table, nil)
	}
	if endgame.Eligible(st) {
		cards, err := chooseFull(table)
		if err != nil {
			return Action{}, err
		}
		preferred := learnedAction(cards)
		if len(table.Hands[st.Trick.TurnSeat]) == 1 {
			return preferred, nil
		}
		options, err := endgame.Choices(st)
		if err != nil {
			return Action{}, err
		}
		best := options[0]
		for _, o := range options[1:] {
			if o.NetLevels > best.NetLevels {
				best = o
			}
		}
		var want [rules.DoubleDeckSize]bool
		for _, c := range cards {
			want[c.ID] = true
		}
		for _, o := range options {
			if o.NetLevels != best.NetLevels || len(o.Cards.CardIDs) != len(cards) {
				continue
			}
			match := true
			for _, id := range o.Cards.CardIDs {
				if !want[id] {
					match = false
					break
				}
			}
			if match {
				return preferred, nil
			}
		}
		return Action{string(game.EvPlayCards), best.Cards}, nil
	}
	var ranked []tacticalChoice
	seen := map[[54]byte]bool{}
	_, err := scoreFull(table, func(cards []rules.Card, score float64) {
		var faces [54]byte
		for _, c := range cards {
			faces[c.ID%54]++
		}
		if !seen[faces] {
			seen[faces] = true
			ranked = append(ranked, tacticalChoice{cards, score})
		}
	})
	if err != nil {
		return Action{}, err
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	best := learnedAction(ranked[0].cards)
	if len(ranked) == 1 {
		return best, nil
	}
	limit := min(search.Candidates, len(ranked))
	if search.Diverse && limit < len(ranked) && limit > 1 {
		diversifyLast(ranked, limit)
	}
	bestValue := ruleOutcome{levels: -4}
	for _, candidate := range ranked[:limit] {
		move := learnedAction(candidate.cards)
		value, e := ruleReturn(st, move.Payload.(game.PlayCardsPayload), search.ExactTail)
		if e != nil {
			return Action{}, e
		}
		if value.better(bestValue, search.PreferPoints) {
			best, bestValue = move, value
			// At the maximum settlement only the optional points tiebreak
			// can improve on this candidate.
			if value.levels == 3 && !search.PreferPoints {
				break
			}
		}
	}
	return best, nil
}

// Reserve the last rollout for the best move with a different suit class,
// number of cards, or point total from those already selected.
func diversifyLast(ranked []tacticalChoice, limit int) {
	type shape struct {
		suit          rules.SuitClass
		count, points int
	}
	signature := func(candidate tacticalChoice) shape {
		suit, same := rules.ComputeSuitClassAllSame(candidate.cards)
		if !same {
			suit = rules.SCMix
		}
		return shape{suit, len(candidate.cards), int(cardPoints(candidate.cards))}
	}
	seen := make(map[shape]bool, limit-1)
	for _, candidate := range ranked[:limit-1] {
		seen[signature(candidate)] = true
	}
	for i := limit - 1; i < len(ranked); i++ {
		if !seen[signature(ranked[i])] {
			ranked[limit-1], ranked[i] = ranked[i], ranked[limit-1]
			return
		}
	}
}
