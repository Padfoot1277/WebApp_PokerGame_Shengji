package bot

import (
	"fmt"
	"math"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// Decide sees the same private view as a seated player, never the full game state.
// Automatic trumps implements the in-game practice mode: call/change/attack whenever legal.
func Decide(s game.ViewState, history []PublicTrick, automaticTrumps bool) (Action, error) {
	seat := s.MySeat
	if seat < 0 || seat >= 4 {
		return Action{}, fmt.Errorf("bot must be seated")
	}
	idle := Action{}
	switch s.Phase {
	case game.PhaseLobby:
		if !s.Seats[seat].Ready {
			return Action{string(game.EvReady), struct{}{}}, nil
		}
	case game.PhaseCallTrump:
		if s.CallPassedSeats[seat] || (s.CallMode != game.CallModeRace && s.CallTurnSeat != seat) {
			return idle, nil
		}
		if automaticTrumps {
			return automaticTrump(s)
		}
		return phaseAction(s)
	case game.PhaseTrumpFight:
		if s.SurrenderVotes[(seat+2)%4] && !s.SurrenderVotes[seat] {
			return Action{string(game.EvSurrender), struct{}{}}, nil
		}
		if s.FightTurnSeat != seat || s.FightPassedSeats[seat] {
			return idle, nil
		}
		if automaticTrumps {
			return automaticTrump(s)
		}
		return phaseAction(s)
	case game.PhaseBottom, game.PhasePlayTrick:
		if s.SurrenderVotes[(seat+2)%4] && !s.SurrenderVotes[seat] {
			return Action{string(game.EvSurrender), struct{}{}}, nil
		}
		if s.Phase == game.PhaseBottom {
			if s.BottomOwnerSeat != seat {
				return idle, nil
			}
			return phaseAction(s)
		}
		if s.Trick.TurnSeat != seat {
			return idle, nil
		}
		cards, err := chooseWithHistory(s, history)
		if err != nil {
			return idle, err
		}
		ids := []int{}
		for _, c := range cards {
			ids = append(ids, c.ID)
		}
		return Action{string(game.EvPlayCards), game.PlayCardsPayload{CardIDs: ids}}, nil
	case game.PhaseRoundSettle:
		if s.NextStarterSeat == seat {
			return Action{string(game.EvStartNextRound), struct{}{}}, nil
		}
	}
	return idle, nil
}
func automaticTrump(s game.ViewState) (Action, error) {
	h := handOf(s)
	used := map[int]bool{}
	for _, id := range s.UsedTrumpIDs {
		used[id] = true
	}
	available := []rules.Card{}
	for _, c := range h {
		if !used[c.ID] {
			available = append(available, c)
		}
	}
	calling := s.Phase == game.PhaseCallTrump
	level := s.Trump.LevelRank
	if calling {
		level = s.Teams[s.Seats[s.MySeat].Team].LevelRank
	}
	// A legal attack always takes precedence over a change.
	if !calling && s.Trump.HasTrumpSuit {
		for i, j := range available {
			for _, k := range available[i+1:] {
				if rules.ValidateAttackTrump(j, k) == nil {
					return Action{string(game.EvAttackTrump), game.AttackTrumpPayload{JokerIDs: []int{j.ID, k.ID}}}, nil
				}
			}
		}
	}
	best := Action{string(game.EvCallPass), struct{}{}}
	score := math.Inf(-1)
	if !calling && s.Trump.Locked {
		return best, nil
	}
	for _, suit := range []rules.Suit{rules.Heart, rules.Diamond, rules.Spade, rules.Club} {
		levels := []rules.Card{}
		count := 0
		for _, c := range h {
			if c.Suit == suit {
				count++
			}
		}
		for _, c := range available {
			if c.Suit == suit && c.Rank == level {
				levels = append(levels, c)
			}
		}
		for _, j := range available {
			locked := false
			if calling {
				var err error
				_, locked, err = rules.ValidateCallTrump(level, j, levels)
				if err != nil {
					continue
				}
			} else {
				if len(levels) != 2 {
					continue
				}
				if _, err := rules.ValidateChangeTrump(level, j, levels[0], levels[1]); err != nil {
					continue
				}
			}
			value := float64(count)*100 + handValue(h, rules.Trump{HasTrumpSuit: true, Suit: suit, LevelRank: level})*0.05
			if locked {
				value += 1
			}
			if value <= score {
				continue
			}
			score = value
			ids := []int{}
			for _, c := range levels {
				ids = append(ids, c.ID)
			}
			if calling {
				best = Action{string(game.EvCallTrump), game.CallTrumpPayload{JokerID: j.ID, LevelIDs: ids}}
			} else {
				best = Action{string(game.EvChangeTrump), game.ChangeTrumpPayload{JokerID: j.ID, LevelIDs: ids}}
			}
		}
	}
	return best, nil
}
