package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
)

func leafDeal(seed int64, actor *bot.LearnModel) ([]bot.LeafTarget, error) {
	st, err := arena.NewRound(seed)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(seed ^ 0x317c))
	play := (policy{actor, "raw"}).play
	var root game.GameState
	count := 0
	_, err = arena.Play(st, func(s game.GameState) ([]int, error) {
		if !endgame.Eligible(s) {
			count++
			if rng.Intn(count) == 0 {
				root = s
			}
		}
		return play(s)
	}, nil)
	if err != nil || count == 0 {
		return nil, err
	}
	seat := root.Trick.TurnSeat
	team := root.Seats[seat].Team
	ranked, err := actor.Rank(bot.Observe(root, seat), 3)
	if err != nil || len(ranked) < 2 {
		return nil, err
	}
	row := make([]bot.LeafTarget, 0, len(ranked))
	for _, move := range ranked {
		next, err := bot.SearchLeaf(root, move.Action, actor)
		if err != nil {
			return nil, err
		}
		final, err := arena.Play(next, play, nil)
		if err != nil {
			return nil, err
		}
		levels := arena.Reward(final, team)
		target := bot.LeafTarget{Levels: levels}
		if next.Phase == game.PhasePlayTrick {
			s := next.Trick.TurnSeat
			target = bot.NextTrickTarget(bot.Observe(next, s), next.Seats[s].Team == team, levels)
		}
		row = append(row, target)
	}
	return row, nil
}

type leafData struct {
	rows                           [][]bot.LeafTarget
	Branches, Sampled, Informative int
}

func collectLeaves(actor *bot.LearnModel, seed int64, deals, workers int) (leafData, error) {
	var data leafData
	for offset := 0; offset < deals; offset += 128 {
		part, err := parallel(min(128, deals-offset), workers, func(i int) ([]bot.LeafTarget, error) { return leafDeal(seed+int64(offset+i), actor) })
		if err != nil {
			return data, err
		}
		for _, row := range part {
			if len(row) < 2 {
				continue
			}
			data.Sampled++
			data.Branches += len(row)
			for _, a := range row[1:] {
				if a.Levels != row[0].Levels {
					data.rows = append(data.rows, row)
					data.Informative++
					break
				}
			}
		}
		if (offset+len(part))%1024 == 0 || offset+len(part) == deals {
			fmt.Printf("seed %d: %d/%d deals, %d informative\n", seed, offset+len(part), deals, data.Informative)
		}
	}
	if data.Informative == 0 {
		return data, fmt.Errorf("no informative candidate sets")
	}
	return data, nil
}

type leafMetric struct{ MissedLevels, Correct, Fixed, Broken, Better, Worse int }

func checkLeaves(m *bot.LearnModel, rows [][]bot.LeafTarget, original []int) leafMetric {
	var out leafMetric
	for i, row := range rows {
		top := row[0].Levels
		for _, a := range row[1:] {
			top = max(top, a.Levels)
		}
		selected := row[m.PickLeaf(row)].Levels
		out.MissedLevels += top - selected
		if selected == top {
			out.Correct++
		}
		if original != nil {
			old := original[i]
			if old < top && selected == top {
				out.Fixed++
			}
			if old == top && selected < top {
				out.Broken++
			}
			if selected > old {
				out.Better++
			}
			if selected < old {
				out.Worse++
			}
		}
	}
	return out
}

func originalSelections(m *bot.LearnModel, rows [][]bot.LeafTarget) []int {
	s := make([]int, len(rows))
	for i, r := range rows {
		s[i] = r[m.PickLeaf(r)].Levels
	}
	return s
}

func leaftrain(args []string) error {
	f := flag.NewFlagSet("leaftrain", flag.ContinueOnError)
	actorPath := f.String("actor", "", "frozen ranking and reply checkpoint")
	valuePath := f.String("value", "", "starting value checkpoint")
	out := f.String("out", "", "new directory")
	seed := f.Int64("seed", 91000000, "first training deal; validation and test start 1M and 2M later")
	deals := f.Int("deals", 8192, "training deals")
	valid := f.Int("validation", 2048, "validation deals")
	test := f.Int("test", 3072, "held-out deals")
	epochs := f.Int("epochs", 160, "passes over the training rows")
	workers := f.Int("workers", 8, "parallel deals")
	rate := f.Float64("rate", 0.0002, "Adam learning rate")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *out == "" || *epochs < 1 || *workers < 1 || !(*rate > 0 && *rate <= 1) || min(*deals, *valid, *test) < 1 || max(*deals, *valid, *test) > 1000000 {
		return fmt.Errorf("invalid training configuration")
	}
	actor, err := load(*actorPath)
	if err != nil {
		return err
	}
	value, err := load(*valuePath)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(*out, 0755); err != nil {
		return err
	}
	path := func(name string) string { return filepath.Join(*out, name) }
	if _, err = save(path("config.json"), map[string]any{"actor": *actorPath, "value": *valuePath, "seed": *seed, "deals": *deals, "validation": *valid, "test": *test, "epochs": *epochs, "rate": *rate, "batch": 32}); err != nil {
		return err
	}
	train, err := collectLeaves(actor, *seed, *deals, *workers)
	if err != nil {
		return err
	}
	val, err := collectLeaves(actor, *seed+1000000, *valid, *workers)
	if err != nil {
		return err
	}
	original := originalSelections(value, val.rows)
	rng := rand.New(rand.NewSource(*seed))
	var history []leafMetric
	var best leafMetric
	bestEpoch := 0
	for epoch := 0; epoch <= *epochs; epoch++ {
		if epoch > 0 {
			rng.Shuffle(len(train.rows), func(i, j int) { train.rows[i], train.rows[j] = train.rows[j], train.rows[i] })
			for i := 0; i < len(train.rows); i += 32 {
				value.FitLeafBatch(train.rows[i:min(i+32, len(train.rows))], *rate)
			}
		}
		score := checkLeaves(value, val.rows, original)
		history = append(history, score)
		if epoch == 0 || score.MissedLevels < best.MissedLevels || score.MissedLevels == best.MissedLevels && score.Correct > best.Correct {
			best, bestEpoch = score, epoch
			if _, err = save(path("best.json"), value); err != nil {
				return err
			}
		}
		if epoch%20 == 0 {
			fmt.Printf("epoch %d/%d, validation missed %d, selected %d\n", epoch, *epochs, score.MissedLevels, bestEpoch)
		}
	}
	if _, err = save(path("final.json"), value); err != nil {
		return err
	}
	selected, err := load(path("best.json"))
	if err != nil {
		return err
	}
	base, err := load(*valuePath)
	if err != nil {
		return err
	}
	held, err := collectLeaves(actor, *seed+2000000, *test, *workers)
	if err != nil {
		return err
	}
	old := originalSelections(base, held.rows)
	_, err = save(path("report.json"), map[string]any{"train": train, "validation": val, "heldout": held, "history": history, "bestEpoch": bestEpoch, "testBase": checkLeaves(base, held.rows, old), "testBest": checkLeaves(selected, held.rows, old), "testFinal": checkLeaves(value, held.rows, old)})
	return err
}
