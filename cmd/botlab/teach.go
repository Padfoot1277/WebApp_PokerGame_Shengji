package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
)

type teacherCounts struct {
	Deals, RepresentedDeals, Eligible, Forced, AllTie, Informative, Covered, Retained int
}

type teacherSet struct {
	counts teacherCounts
	rows   []bot.ExactSample
}

func collectTeacherDeal(visitor *bot.LearnModel, seed int64, training bool) (teacherSet, error) {
	st, err := arena.NewRound(seed)
	if err != nil {
		return teacherSet{}, err
	}
	set := teacherSet{counts: teacherCounts{Deals: 1}}
	p := policy{visitor, "raw"}
	_, err = arena.Play(st, func(s game.GameState) ([]int, error) {
		played, err := p.play(s)
		if err != nil || !endgame.Eligible(s) {
			return played, err
		}
		set.counts.Eligible++
		if len(s.Seats[s.Trick.TurnSeat].Hand) == 1 {
			set.counts.Forced++
			return played, nil
		}
		options, e := endgame.Choices(s)
		if e != nil {
			return nil, e
		}
		if len(options) == 1 {
			set.counts.Forced++
			return played, nil
		}
		worst, best := options[0].NetLevels, options[0].NetLevels
		for _, o := range options {
			worst = min(worst, o.NetLevels)
			best = max(best, o.NetLevels)
		}
		if worst == best {
			set.counts.AllTie++
			return played, nil
		}
		set.counts.Informative++
		sample, e := visitor.LabelExact(bot.Observe(s, s.Trick.TurnSeat), options)
		if e != nil {
			return nil, e
		}
		if sample.Covered() {
			set.counts.Covered++
		}
		// Validation retains all globally informative states, including
		// cases where the ranked candidate list cannot attain the optimum.
		if !training || sample.Informative() && sample.Covered() {
			set.rows = append(set.rows, sample)
		}
		return played, nil
	}, nil)
	if err != nil {
		return set, fmt.Errorf("deal seed %d: %w", seed, err)
	}
	if set.counts.Informative > 0 {
		set.counts.RepresentedDeals = 1
	}
	set.counts.Retained = len(set.rows)
	return set, nil
}

func collectTeacherSet(visitor *bot.LearnModel, start int64, deals, workers int, training bool) (teacherSet, error) {
	var set teacherSet
	for offset := 0; offset < deals; offset += 256 {
		n := min(256, deals-offset)
		part, e := parallel(n, workers, func(i int) (teacherSet, error) { return collectTeacherDeal(visitor, start+int64(offset+i), training) })
		if e != nil {
			return set, e
		}
		for _, d := range part {
			s := &set.counts
			s.Deals++
			s.RepresentedDeals += d.counts.RepresentedDeals
			s.Eligible += d.counts.Eligible
			s.Forced += d.counts.Forced
			s.AllTie += d.counts.AllTie
			s.Informative += d.counts.Informative
			s.Covered += d.counts.Covered
			s.Retained += len(d.rows)
			set.rows = append(set.rows, d.rows...)
		}
		fmt.Printf("seed %d: %d/%d deals, %d labeled decisions retained\n", start, offset+n, deals, len(set.rows))
	}
	if len(set.rows) == 0 {
		return set, fmt.Errorf("no informative examples on seed range starting %d", start)
	}
	return set, nil
}

type teacherMetric struct{ Decisions, Correct, MissedNetLevels int }

func scoreTeacher(model *bot.LearnModel, data []bot.ExactSample) (teacherMetric, error) {
	m := teacherMetric{Decisions: len(data)}
	for _, s := range data {
		regret, e := model.ExactRegret(s)
		if e != nil {
			return m, e
		}
		m.MissedNetLevels += regret
		if regret == 0 {
			m.Correct++
		}
	}
	return m, nil
}

type teacherEpoch struct {
	Epoch, Updates        int
	MeanBatchCrossEntropy *float64
	Train, Validation     teacherMetric
}

func teach(args []string) error {
	f := flag.NewFlagSet("teach", flag.ContinueOnError)
	base := f.String("model", "training/run-01/round-12.json", "fixed visitor and initial weights")
	out := f.String("out", "training/teacher", "fresh output directory")
	deals := f.Int("deals", 4096, "training deals")
	validation := f.Int("validation", 768, "validation deals")
	seed := f.Int64("seed", 31000000, "first training deal seed")
	validSeed := f.Int64("validation-seed", 32000000, "first validation deal seed")
	epochs := f.Int("epochs", 20, "number of passes over the fixed training examples")
	batch := f.Int("batch", 32, "minibatch size")
	rate := f.Float64("rate", 0.002, "Adam learning rate")
	shuffle := f.Int64("shuffle", 92831, "example permutation seed")
	workers := f.Int("workers", 8, "concurrent deal collectors")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || *deals <= 0 || *validation <= 0 || *epochs <= 0 || *batch <= 0 || *workers <= 0 || *rate <= 0 || *rate > 1 || math.IsNaN(*rate) {
		return fmt.Errorf("invalid teacher options")
	}
	if *seed > math.MaxInt64-int64(*deals) || *validSeed > math.MaxInt64-int64(*validation) || *seed < *validSeed+int64(*validation) && *validSeed < *seed+int64(*deals) {
		return fmt.Errorf("training and validation require disjoint, finite seed intervals")
	}
	model, err := load(*base)
	if err != nil {
		return err
	}
	source, err := os.ReadFile(*base)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(*out, 0755); err != nil {
		return fmt.Errorf("fresh teacher output directory: %w", err)
	}
	config := map[string]any{"source": *base, "sourceSHA256": fmt.Sprintf("%x", sha256.Sum256(source)), "trainSeed": *seed, "trainDeals": *deals, "validationSeed": *validSeed, "validationDeals": *validation, "epochs": *epochs, "batch": *batch, "rate": *rate, "shuffle": *shuffle, "workers": *workers, "visitor": "source checkpoint, raw greedy at all four seats", "selection": "lowest validation summed net-level regret, then most optimal decisions, then earliest epoch, including epoch 0"}
	if _, err = save(filepath.Join(*out, "config.json"), config); err != nil {
		return err
	}
	train, err := collectTeacherSet(model, *seed, *deals, *workers, true)
	if err != nil {
		return err
	}
	valid, err := collectTeacherSet(model, *validSeed, *validation, *workers, false)
	if err != nil {
		return err
	}
	return fitTeacher(model, train.rows, valid.rows, train.counts, valid.counts, *out, *epochs, *batch, *shuffle, *rate)
}

// Both label sources use the same actor-only optimizer and validation selection.
func fitTeacher(model *bot.LearnModel, train, valid []bot.ExactSample, trainCounts, validCounts any, out string, epochs, batch int, shuffle int64, rate float64) error {
	rng := rand.New(rand.NewSource(shuffle))
	var err error
	var history []teacherEpoch
	bestEpoch, updates := 0, 0
	var best teacherMetric
	for epoch := 0; epoch <= epochs; epoch++ {
		entry := teacherEpoch{Epoch: epoch}
		if epoch > 0 {
			rng.Shuffle(len(train), func(i, j int) { train[i], train[j] = train[j], train[i] })
			loss := 0.0
			for i := 0; i < len(train); i += batch {
				part := train[i:min(i+batch, len(train))]
				ce, e := model.UpdateExact(part, rate)
				if e != nil {
					return e
				}
				loss += ce * float64(len(part))
				updates++
			}
			loss /= float64(len(train))
			entry.MeanBatchCrossEntropy = &loss
		}
		entry.Updates = updates
		entry.Train, err = scoreTeacher(model, train)
		if err != nil {
			return err
		}
		entry.Validation, err = scoreTeacher(model, valid)
		if err != nil {
			return err
		}
		v := entry.Validation
		if epoch == 0 || v.MissedNetLevels < best.MissedNetLevels || v.MissedNetLevels == best.MissedNetLevels && v.Correct > best.Correct {
			bestEpoch, best = epoch, v
			if _, err = save(filepath.Join(out, "best.json"), model); err != nil {
				return err
			}
		}
		fmt.Printf("epoch %d/%d, validation optimal %d/%d, missed levels %d, selected epoch %d\n", epoch, epochs, v.Correct, v.Decisions, v.MissedNetLevels, bestEpoch)
		history = append(history, entry)
		if _, err = save(filepath.Join(out, "report.json"), map[string]any{"trainCounts": trainCounts, "validationCounts": validCounts, "epochs": history, "bestEpoch": bestEpoch}); err != nil {
			return err
		}
	}
	_, err = save(filepath.Join(out, "final.json"), model)
	return err
}
