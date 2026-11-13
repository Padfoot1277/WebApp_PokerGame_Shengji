package main

import (
	"flag"
	"fmt"
	"math"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
)

type matchLeg struct {
	ModelTeam, WinnerTeam, Rounds int
	FinalTeams                    [2]game.TeamState
}

type matchPair struct {
	Seed      int64
	Legs      [2]matchLeg
	ModelWins int
}

func match(args []string) error {
	f := flag.NewFlagSet("match", flag.ContinueOnError)
	model := f.String("model", "", policyHelp)
	opp := f.String("opponent", "legacy", policyHelp)
	mode := f.String("mode", "search", "search, hybrid, or raw")
	valuePath := f.String("search-value", "", "separate value checkpoint for the evaluated search team")
	opMode := f.String("opponent-mode", "search", "search, hybrid, or raw for another checkpoint")
	pairs := f.Int("pairs", 12, "paired full matches to A")
	workers := f.Int("workers", 4, "concurrent pairs")
	seed := f.Int64("seed", 9500000, "first held-out match seed")
	out := f.String("out", "", "report file")
	if err := f.Parse(args); err != nil {
		return err
	}
	validMode := func(s string) bool { return s == "search" || s == "raw" || s == "hybrid" }
	if f.NArg() > 0 || *model == "" || *out == "" || *pairs < 2 || *workers < 1 || !validMode(*mode) || !validMode(*opMode) {
		return fmt.Errorf("match requires a model, output, valid policy modes, at least two pairs and positive worker count")
	}
	a, err := loadPolicy(*model, *mode)
	if err != nil {
		return err
	}
	m := a.model
	playModel := a.play
	if *valuePath != "" {
		if *mode != "search" || m == nil {
			return fmt.Errorf("-search-value requires search mode")
		}
		value, err := load(*valuePath)
		if err != nil {
			return err
		}
		playModel = func(st game.GameState) ([]int, error) {
			action, err := bot.SearchWithModels(st, m, m, value)
			if err != nil {
				return nil, err
			}
			return ids(action)
		}
	}
	b, err := loadPolicy(*opp, *opMode)
	if err != nil {
		return err
	}
	results, err := parallel(*pairs, *workers, func(i int) (matchPair, error) {
		p := matchPair{Seed: *seed + int64(i)}
		for team := range p.Legs {
			final, err := arena.Match(p.Seed, func(st game.GameState) ([]int, error) {
				if st.Seats[st.Trick.TurnSeat].Team == team {
					return playModel(st)
				}
				return b.play(st)
			})
			if err != nil {
				return p, err
			}
			p.Legs[team] = matchLeg{team, final.WinnerTeam, final.RoundIndex + 1, final.Teams}
			if final.WinnerTeam == team {
				p.ModelWins++
			}
		}
		return p, nil
	})
	if err != nil {
		return err
	}
	var wins, rounds int
	for _, p := range results {
		wins += p.ModelWins
		rounds += p.Legs[0].Rounds + p.Legs[1].Rounds
	}
	games := 2 * len(results)
	// Treat each independent deal stream as one [0,1] observation and keep
	// the two correlated seat swaps together. Hoeffding's two-sided bound
	// remains conservative even if every observed seed gives the same result.
	rate := float64(wins) / float64(games)
	margin := math.Sqrt(math.Log(2/0.05) / (2 * float64(len(results))))
	ci := [2]float64{max(0, rate-margin), min(1, rate+margin)}
	_, err = save(*out, map[string]any{
		"model": *model, "mode": a.mode, "searchValue": *valuePath, "opponent": *opp, "opponentMode": b.mode,
		"seed": *seed, "pairs": *pairs, "matches": games, "wins": wins, "rounds": rounds,
		"winRate": rate, "pairedHoeffding95": ci, "results": results,
	})
	if err != nil {
		return err
	}
	fmt.Printf("%d wins in %d matches to A (%d rounds), win rate %.3f, paired Hoeffding 95%% [%.3f, %.3f]\n", wins, games, rounds, rate, ci[0], ci[1])
	return nil
}
