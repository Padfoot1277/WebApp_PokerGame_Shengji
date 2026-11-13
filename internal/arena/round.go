// Package arena plays offline rounds with the same command reducer as the room.
package arena

import (
	"fmt"
	"math/rand"
	"strconv"

	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// NewRound samples a post-deal ordered calling window. Both teams' levels and
// the opening seat are sampled to cover the range of mid-match rounds. Every
// decision from calling through discarding is made by the production baseline.
func NewRound(seed int64) (game.GameState, error) {
	rng := rand.New(rand.NewSource(seed))
	deck := shuffledDeck(rng)
	starter := rng.Intn(4)
	var teams [2]game.TeamState
	for i := range teams {
		teams[i].LevelRank = rules.AddRank(rules.R2, rng.Intn(12))
	}
	return openRound(seed, deck, 1, teams, starter, nil)
}

func shuffledDeck(rng *rand.Rand) []rules.Card {
	deck := rules.NewDoubleDeck()
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	return deck
}

func openRound(seed int64, deck []rules.Card, index int, teams [2]game.TeamState, starter int, raceOrder []int) (game.GameState, error) {
	hands, bottom := rules.Deal(deck)
	st := game.GameState{
		Phase: game.PhaseCallTrump, RoundIndex: index, Teams: teams, CallMode: game.CallModeOrdered,
		NextStarterSeat: starter, CallerSeat: starter, CallTurnSeat: starter,
		FightTurnSeat: -1, WinnerTeam: -1, BottomOwnerSeat: -1,
		Bottom: bottom, BottomCount: len(bottom), UsedTrumpIDs: map[int]bool{},
		Trump: game.TrumpState{Trump: rules.Trump{LevelRank: rules.RPending}, CallerSeat: -1},
	}
	if index == 0 {
		st.CallMode = game.CallModeRace
		st.CallerSeat, st.CallTurnSeat = -1, -1
	}
	for i, h := range hands {
		team := game.TeamOfSeat(i)
		t := rules.Trump{LevelRank: st.Teams[team].LevelRank}
		for j := range h {
			h[j].SuitClass = rules.ComputeSuitClass(h[j], t)
		}
		rules.SortHand(h, t)
		st.Seats[i] = game.SeatState{UID: strconv.Itoa(i), Ready: true, Online: true, Team: team, Hand: h, HandCount: len(h)}
	}
	for count := 0; st.Phase != game.PhasePlayTrick; count++ {
		if count >= 64 {
			return st, fmt.Errorf("seed %d: opening exceeded 64 events", seed)
		}
		var seat int
		switch st.Phase {
		case game.PhaseCallTrump:
			seat = st.CallTurnSeat
			if st.CallMode == game.CallModeRace {
				seat = raceOrder[st.CallPassCount]
			}
		case game.PhaseBottom:
			seat = st.BottomOwnerSeat
		case game.PhaseTrumpFight:
			seat = st.FightTurnSeat
		default:
			return st, fmt.Errorf("seed %d: unexpected phase %s", seed, st.Phase)
		}
		act, err := bot.DecideFull(bot.Observe(st, seat), nil)
		if err != nil {
			return st, fmt.Errorf("seed %d seat %d %s: %w", seed, seat, st.Phase, err)
		}
		res, problem := game.Reduce(st, st.Seats[seat].UID, game.ClientEventType(act.Type), act.Payload)
		if problem != nil {
			return st, fmt.Errorf("seed %d seat %d %s: %w", seed, seat, act.Type, problem)
		}
		st = res.State
	}
	return st, nil
}

// Step applies an intent, including the game's throw-blocking and settlement.
// It leaves the input state available to evaluate another branch.
func Step(st game.GameState, ids []int) (game.GameState, error) {
	seat := st.Trick.TurnSeat
	if st.Phase != game.PhasePlayTrick || seat < 0 || seat >= len(st.Seats) {
		return st, fmt.Errorf("cannot play in phase %s seat %d", st.Phase, seat)
	}
	res, err := game.Reduce(st, st.Seats[seat].UID, game.EvPlayCards, game.PlayCardsPayload{CardIDs: ids})
	if err != nil {
		return st, fmt.Errorf("trick %d seat %d cards %v: %w", st.TrickIndex, seat, ids, err)
	}
	return res.State, nil
}

// Play takes a read-only state in its decision callback. A trace, if requested,
// should start with the seed that produced the initial state via NewRound.
func Play(st game.GameState, decide func(game.GameState) ([]int, error), trace *Replay) (game.GameState, error) {
	for count := 0; st.Phase == game.PhasePlayTrick; count++ {
		if count >= 100 {
			return st, fmt.Errorf("exceeded 100 plays")
		}
		ids, err := decide(st)
		if err != nil {
			return st, fmt.Errorf("trick %d seat %d: %w", st.TrickIndex, st.Trick.TurnSeat, err)
		}
		next, err := Step(st, ids)
		if err != nil {
			return st, err
		}
		if trace != nil {
			trace.Moves = append(trace.Moves, Move{Trick: st.TrickIndex, Seat: st.Trick.TurnSeat, CardIDs: append([]int(nil), ids...)})
		}
		st = next
	}
	if st.Phase != game.PhaseRoundSettle && st.Phase != game.PhaseGameOver {
		return st, fmt.Errorf("unexpected final phase %s", st.Phase)
	}
	return st, nil
}

// Reward is the awarded level difference for team at a settled round.
func Reward(final game.GameState, team int) int {
	diff := final.CallerDelta - final.DefenderDelta
	if final.Seats[final.CallerSeat].Team == team {
		return diff
	}
	return -diff
}

type Move struct {
	Trick   int   `json:"trick"`
	Seat    int   `json:"seat"`
	CardIDs []int `json:"cardIds"`
}

type Replay struct {
	Seed  int64  `json:"seed"`
	Moves []Move `json:"moves"`
}

// Verify regenerates the opening and feeds the recorded intents through the
// real reducer without running any play policy.
func (r Replay) Verify() (game.GameState, error) {
	st, err := NewRound(r.Seed)
	if err != nil {
		return st, err
	}
	for i, move := range r.Moves {
		if st.TrickIndex != move.Trick || st.Trick.TurnSeat != move.Seat {
			return st, fmt.Errorf("replay diverged at move %d", i)
		}
		st, err = Step(st, move.CardIDs)
		if err != nil {
			return st, fmt.Errorf("replay move %d: %w", i, err)
		}
	}
	if st.Phase != game.PhaseRoundSettle && st.Phase != game.PhaseGameOver {
		return st, fmt.Errorf("replay incomplete after %d moves", len(r.Moves))
	}
	return st, nil
}
