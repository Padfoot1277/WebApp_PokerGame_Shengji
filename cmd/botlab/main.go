// botlab trains and evaluates the full-information play policy against the Go engine.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
)

// Jobs occupy fixed output slots. Scheduling cannot change gradient ordering.
func parallel[T any](count, workers int, job func(int) (T, error)) ([]T, error) {
	if count < 1 || workers < 1 {
		return nil, fmt.Errorf("positive job and worker counts required")
	}
	out := make([]T, count)
	errs := make([]error, count)
	queue := make(chan int)
	var wg sync.WaitGroup
	for range min(workers, count) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				out[i], errs[i] = job(i)
			}
		}()
	}
	for i := range count {
		queue <- i
	}
	close(queue)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("job %d: %w", i, err)
		}
	}
	return out, nil
}

func save(path string, value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".botlab-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	closeErr := tmp.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func load(path string) (*bot.LearnModel, error) {
	return bot.LoadLearnedPolicy(path)
}

func ids(a bot.Action) ([]int, error) {
	p, ok := a.Payload.(game.PlayCardsPayload)
	if !ok || a.Type != string(game.EvPlayCards) || len(p.CardIDs) == 0 {
		return nil, fmt.Errorf("policy returned invalid play %q", a.Type)
	}
	return p.CardIDs, nil
}

type episode struct {
	samples []bot.LearnSample
	points  int
}

func selfPlay(seed int64, model *bot.LearnModel) (episode, error) {
	st, err := arena.NewRound(seed)
	if err != nil {
		return episode{}, err
	}
	rng := rand.New(rand.NewSource(seed ^ 0x4ad7e1))
	var ep episode
	var teams []int
	final, err := arena.Play(st, func(s game.GameState) ([]int, error) {
		seat := s.Trick.TurnSeat
		action, sample, err := model.Play(bot.Observe(s, seat), rng)
		if err != nil {
			return nil, err
		}
		ep.samples = append(ep.samples, sample)
		teams = append(teams, s.Seats[seat].Team)
		return ids(action)
	}, nil)
	if err != nil {
		return ep, fmt.Errorf("seed %d: %w", seed, err)
	}
	for i, team := range teams {
		ep.samples[i].Reward = float64(arena.Reward(final, team)) / 3
	}
	ep.points = final.RoundPointsFinal
	return ep, nil
}

type iteration struct {
	Round, Games, Updates, Decisions int
	FirstSeed                        int64
	MeanAttackerPoints               float64
	SHA256                           string
}

func train(args []string) error {
	f := flag.NewFlagSet("train", flag.ContinueOnError)
	out := f.String("out", "training/run-01", "new output directory")
	rounds := f.Int("rounds", 12, "training iterations")
	batches := f.Int("batches", 4, "fresh on-policy batches per iteration")
	games := f.Int("games", 16, "complete self-play rounds per batch")
	workers := f.Int("workers", 8, "parallel games")
	seed := f.Int64("seed", 1311000, "first deal seed")
	modelSeed := f.Int64("model-seed", 731, "weight initialization seed")
	variant := f.String("variant", "baseline", "initial architecture when training without a checkpoint: baseline, wide, rich, tactical")
	startModel := f.String("start-model", "", "start with checkpoint weights and fresh optimizer moments")
	expandTactical := f.Bool("expand-tactical", false, "extend a starting baseline checkpoint with tactical features")
	finalOnly := f.Bool("final-only", false, "save the final checkpoint only and retain all iteration hashes")
	checkpointEvery := f.Int("checkpoint-every", 1, "save weights every N iterations and at the final iteration")
	rate := f.Float64("rate", 0.005, "optimizer learning rate")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *rounds <= 0 || *batches <= 0 || *games <= 0 || *rate <= 0 || *workers <= 0 || *checkpointEvery <= 0 {
		return fmt.Errorf("training parameters must be positive")
	}
	var model *bot.LearnModel
	var err error
	config := map[string]any{"firstDealSeed": *seed, "iterations": *rounds, "batchesPerIteration": *batches, "gamesPerBatch": *games, "learningRate": *rate, "workers": *workers, "reward": "acting team net round level delta / 3"}
	config["checkpointEvery"] = *checkpointEvery
	config["finalOnly"] = *finalOnly
	if *startModel == "" {
		if *expandTactical {
			return fmt.Errorf("-expand-tactical requires -start-model")
		}
		config["modelSeed"] = *modelSeed
		model, err = bot.NewLearnVariant(*modelSeed, *variant)
	} else {
		raw, e := os.ReadFile(*startModel)
		if e != nil {
			return e
		}
		model = new(bot.LearnModel)
		if e := json.Unmarshal(raw, model); e != nil {
			return e
		}
		if e := model.Validate(); e != nil {
			return e
		}
		config["startModel"] = *startModel
		config["sourceSHA256"] = fmt.Sprintf("%x", sha256.Sum256(raw))
		config["freshOptimizer"] = true
		config["expandTactical"] = *expandTactical
		if *expandTactical {
			model, err = bot.ExpandTactical(model)
		}
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(*out, 0755); err != nil {
		return fmt.Errorf("need a fresh run directory: %w", err)
	}
	if _, err := save(filepath.Join(*out, "initial.json"), model); err != nil {
		return err
	}
	config["variant"] = model.Variant
	if model.Variant == "" {
		config["variant"] = "baseline"
	}
	config["parameters"] = len(model.Weights)
	if _, err := save(filepath.Join(*out, "config.json"), config); err != nil {
		return err
	}
	var history []iteration
	nextSeed := *seed
	for r := 1; r <= *rounds; r++ {
		rec := iteration{Round: r, FirstSeed: nextSeed}
		for range *batches {
			episodes, err := parallel(*games, *workers, func(i int) (episode, error) { return selfPlay(nextSeed+int64(i), model) })
			if err != nil {
				return err
			}
			var samples []bot.LearnSample
			for _, ep := range episodes {
				samples = append(samples, ep.samples...)
				rec.MeanAttackerPoints += float64(ep.points)
			}
			if err := model.Update(samples, *rate); err != nil {
				return err
			}
			rec.Decisions += len(samples)
			rec.Updates++
			rec.Games += len(episodes)
			nextSeed += int64(len(episodes))
		}
		rec.MeanAttackerPoints /= float64(rec.Games)
		path := filepath.Join(*out, fmt.Sprintf("round-%02d.json", r))
		var hash string
		if r != *rounds && (*finalOnly || r%*checkpointEvery != 0) {
			data, e := json.MarshalIndent(model, "", "  ")
			if e != nil {
				return e
			}
			hash = fmt.Sprintf("%x", sha256.Sum256(append(data, '\n')))
		} else {
			hash, err = save(path, model)
		}
		if err != nil {
			return err
		}
		rec.SHA256 = hash
		history = append(history, rec)
		if _, err := save(filepath.Join(*out, "history.json"), history); err != nil {
			return err
		}
		fmt.Printf("iteration %d/%d, %d games, %d decisions, %d updates, sha256 %.12s\n", r, *rounds, rec.Games, rec.Decisions, rec.Updates, hash)
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: botlab train|teach|leaftrain|midtrain|mideval|calibrate|eval|match|replay|diagnose|funnel [flags]")
	}
	var err error
	switch os.Args[1] {
	case "leaftrain":
		err = leaftrain(os.Args[2:])
	case "train":
		err = train(os.Args[2:])
	case "teach":
		err = teach(os.Args[2:])
	case "midtrain":
		err = midtrain(os.Args[2:])
	case "mideval":
		err = mideval(os.Args[2:])
	case "calibrate":
		err = calibrate(os.Args[2:])
	case "eval":
		err = evaluate(os.Args[2:])
	case "match":
		err = match(os.Args[2:])
	case "replay":
		err = replay(os.Args[2:])
	case "diagnose":
		err = diagnose(os.Args[2:])
	case "funnel":
		err = funnel(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}
