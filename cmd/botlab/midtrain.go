package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"upgrade-lan/internal/bot"
)

type midCounts struct {
	Deals, Positions, TotalCandidates, Branches, Forced, AllTie, Informative, ReferenceSuboptimal int
}

func collectMidSet(visitor *bot.LearnModel, start int64, count, workers int, lateTop3 bool) ([]bot.ExactSample, midCounts, error) {
	var rows []bot.ExactSample
	var stats midCounts
	for offset := 0; offset < count; offset += 128 {
		n := min(128, count-offset)
		part, err := parallel(n, workers, func(i int) (branchDeal, error) {
			return collectBranchDeal(visitor, start+int64(offset+i), lateTop3)
		})
		if err != nil {
			return nil, stats, err
		}
		for _, d := range part {
			stats.Deals++
			if len(d.Choices) == 0 {
				continue
			}
			stats.Positions++
			stats.Branches += len(d.Choices)
			stats.TotalCandidates += d.Candidates
			if len(d.Choices) == 1 {
				stats.Forced++
				continue
			}
			s, err := visitor.LabelRollouts(d.Table, d.Choices)
			if err != nil {
				return nil, stats, err
			}
			if !s.Informative() {
				stats.AllTie++
				continue
			}
			rows = append(rows, s)
			stats.Informative++
			for _, choice := range d.Choices[1:] {
				if choice.NetLevels > d.Choices[0].NetLevels {
					stats.ReferenceSuboptimal++
					break
				}
			}
		}
		fmt.Printf("seed %d: %d/%d deals, %d branches, %d informative positions\n", start, stats.Deals, count, stats.Branches, stats.Informative)
	}
	if len(rows) == 0 {
		return nil, stats, fmt.Errorf("no informative midgame decisions for first seed %d", start)
	}
	return rows, stats, nil
}

func midtrain(args []string) error {
	f := flag.NewFlagSet("midtrain", flag.ContinueOnError)
	base := f.String("model", "training/teacher/best.json", "frozen visitor and starting weights")
	out := f.String("out", "training/midplay-run", "fresh output directory")
	deals := f.Int("deals", 2048, "number of training deals")
	validDeals := f.Int("validation", 512, "number of validation deals")
	seed := f.Int64("seed", 41000000, "first training deal")
	validSeed := f.Int64("validation-seed", 42000000, "first validation deal")
	epochs := f.Int("epochs", 12, "passes over informative training positions")
	batch := f.Int("batch", 32, "minibatch size")
	rate := f.Float64("rate", .001, "Adam learning rate")
	shuffle := f.Int64("shuffle", 19411, "training order seed")
	workers := f.Int("workers", 8, "concurrent deals")
	lateTop3 := f.Bool("late-top3", false, "train on the source policy's three highest-ranked candidates with four to six cards remaining")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || *deals < 1 || *validDeals < 1 || *workers < 1 || *epochs < 1 || *batch < 1 || !(*rate > 0 && *rate <= 1) {
		return fmt.Errorf("invalid midtrain options")
	}
	if *seed > math.MaxInt64-int64(*deals) || *validSeed > math.MaxInt64-int64(*validDeals) || *seed < *validSeed+int64(*validDeals) && *validSeed < *seed+int64(*deals) {
		return fmt.Errorf("training and validation seed intervals must be finite and disjoint")
	}
	model, err := load(*base)
	if err != nil {
		return err
	}
	source, err := os.ReadFile(*base)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(*out, 0755); err != nil {
		return fmt.Errorf("fresh midtrain directory: %w", err)
	}
	config := map[string]any{
		"lateTop3": *lateTop3,
		"source":   *base, "sourceSHA256": fmt.Sprintf("%x", sha256.Sum256(source)),
		"trainSeed": *seed, "trainDeals": *deals, "validationSeed": *validSeed, "validationDeals": *validDeals,
		"epochs": *epochs, "batch": *batch, "rate": *rate, "workers": *workers, "shuffle": *shuffle,
		"target":    "one greedy frozen-policy continuation per root move, actor team net levels",
		"loss":      "conditional cross entropy over evaluated moves, uniform over tied best returns",
		"selection": "smallest total shortlist regret, then most correct on informative validation positions, then earliest epoch including epoch zero",
	}
	if _, err := save(filepath.Join(*out, "config.json"), config); err != nil {
		return err
	}
	train, tc, err := collectMidSet(model, *seed, *deals, *workers, *lateTop3)
	if err != nil {
		return err
	}
	valid, vc, err := collectMidSet(model, *validSeed, *validDeals, *workers, *lateTop3)
	if err != nil {
		return err
	}
	return fitTeacher(model, train, valid, tc, vc, *out, *epochs, *batch, *shuffle, *rate)
}
