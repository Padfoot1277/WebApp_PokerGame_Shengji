// Package endgame solves small perfect-information play positions to round settlement.
package endgame

import (
	"fmt"

	"upgrade-lan/internal/game"
)

const MaxHand = 3

type Choice struct {
	Cards     game.PlayCardsPayload
	NetLevels int // from the team to act at the root
}

func Eligible(st game.GameState) bool {
	if st.Phase != game.PhasePlayTrick || st.Trick.TurnSeat < 0 || st.Trick.TurnSeat >= 4 {
		return false
	}
	for _, seat := range st.Seats {
		if len(seat.Hand) > MaxHand {
			return false
		}
	}
	return len(st.Seats[st.Trick.TurnSeat].Hand) > 0
}

// For small hands, enumerate every nonempty subset. Reduce validates follows
// and applies the actual clipped lead on blocked throws.
func children(st game.GameState, visit func(game.PlayCardsPayload, game.GameState) bool) {
	s := st.Seats[st.Trick.TurnSeat]
	for mask := 1; mask < 1<<len(s.Hand); mask++ {
		p := game.PlayCardsPayload{}
		for i, c := range s.Hand {
			if mask&(1<<i) != 0 {
				p.CardIDs = append(p.CardIDs, c.ID)
			}
		}
		result, err := game.Reduce(st, s.UID, game.EvPlayCards, p)
		if err == nil && !visit(p, result.State) {
			return
		}
	}
}

func score(st game.GameState, team, alpha, beta int) (int, error) {
	if st.Phase == game.PhaseRoundSettle || st.Phase == game.PhaseGameOver {
		v := st.CallerDelta - st.DefenderDelta
		if st.Seats[st.CallerSeat].Team != team {
			v = -v
		}
		return v, nil
	}
	maximize := st.Seats[st.Trick.TurnSeat].Team == team
	best := 4
	if maximize {
		best = -4
	}
	var failure error
	children(st, func(_ game.PlayCardsPayload, next game.GameState) bool {
		var value int
		value, failure = score(next, team, alpha, beta)
		if failure != nil {
			return false
		}
		if maximize {
			best = max(best, value)
			alpha = max(alpha, best)
		} else {
			best = min(best, value)
			beta = min(beta, best)
		}
		return alpha < beta
	})
	if failure == nil && (best < -3 || best > 3) {
		failure = fmt.Errorf("no legal continuation for seat %d", st.Trick.TurnSeat)
	}
	return best, failure
}

// Value returns the partnership's exact net levels under best play on both
// sides. Callers comparing continuations only need the root value, so the
// root moves can share an alpha-beta window.
func Value(st game.GameState, team int) (int, error) {
	if !Eligible(st) || st.CallerSeat < 0 || st.CallerSeat >= 4 || team < 0 || team > 1 {
		return 0, fmt.Errorf("invalid small endgame position or team")
	}
	return score(st, team, -4, 4)
}

// Choices returns the exact minimax value of each accepted play, including
// ties. In particular an inferior action never receives a pruning bound.
func Choices(st game.GameState) ([]Choice, error) {
	if !Eligible(st) || st.CallerSeat < 0 || st.CallerSeat >= 4 {
		return nil, fmt.Errorf("expected a play position with at most %d cards per seat and a valid caller", MaxHand)
	}
	team := st.Seats[st.Trick.TurnSeat].Team
	var out []Choice
	var failure error
	children(st, func(p game.PlayCardsPayload, next game.GameState) bool {
		value, err := score(next, team, -4, 4)
		if err != nil {
			failure = err
			return false
		}
		out = append(out, Choice{p, value})
		return true
	})
	if failure == nil && len(out) == 0 {
		failure = fmt.Errorf("no legal plays")
	}
	return out, failure
}

func Best(st game.GameState) (game.PlayCardsPayload, int, error) {
	options, err := Choices(st)
	if err != nil {
		return game.PlayCardsPayload{}, 0, err
	}
	best := options[0]
	for _, opt := range options[1:] {
		if opt.NetLevels > best.NetLevels {
			best = opt
		}
	}
	return best.Cards, best.NetLevels, nil
}
