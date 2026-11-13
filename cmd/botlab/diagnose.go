package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"slices"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

type diagnosticCounts struct {
	Eligible, Forced, AllChoicesTie, Informative, MultipleBest int
	Optimal, MissedNetLevels                                   [3]int // only on informative positions
	OptimalCandidateAvailable, CandidateGapNetLevels           int
	Corrected, NewlyMissed, LowerRegret, HigherRegret          [2]int // versus baseline, only on informative positions
}

type diagnosticExample struct {
	Seed                                    int64
	PlayIndex, Seat, AttackerPoints         int
	Hands                                   [4][]int
	HandNames                               [4]string
	Trump                                   rules.Trump
	Bottom                                  string
	CallerSeat, BottomOwnerSeat, LeaderSeat int
	TableCards                              [4]string
	Choices                                 []endgame.Choice
	Picked                                  [3][]int
	Regret                                  [3]int
}

type diagnosticDeal struct {
	diagnosticCounts
	Seed    int64
	Example *diagnosticExample
}

func evaluateEndings(seed int64, models [3]*bot.LearnModel) (diagnosticDeal, error) {
	result := diagnosticDeal{Seed: seed}
	st, err := arena.NewRound(seed)
	if err != nil {
		return result, err
	}
	visitor := policy{models[0], "raw"}
	for turn := 0; st.Phase == game.PhasePlayTrick; turn++ {
		if turn >= 100 {
			return result, fmt.Errorf("seed %d exceeded 100 plays", seed)
		}
		played, err := visitor.play(st)
		if err != nil {
			return result, err
		}
		if endgame.Eligible(st) {
			result.Eligible++
			// Singletons have exactly one legal play in a reachable position.
			if len(st.Seats[st.Trick.TurnSeat].Hand) == 1 {
				result.Forced++
			} else if err := inspectEnding(&result, st, turn, played, models); err != nil {
				return result, err
			}
		}
		st, err = arena.Step(st, played)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func inspectEnding(d *diagnosticDeal, st game.GameState, turn int, basePlay []int, models [3]*bot.LearnModel) error {
	opts, err := endgame.Choices(st)
	if err != nil {
		return err
	}
	if len(opts) == 1 {
		d.Forced++
		return nil
	}
	best, worst := opts[0].NetLevels, opts[0].NetLevels
	for _, o := range opts {
		best = max(best, o.NetLevels)
		worst = min(worst, o.NetLevels)
	}
	if best == worst {
		d.AllChoicesTie++
		return nil
	}
	d.Informative++
	countBest := 0
	for _, o := range opts {
		if o.NetLevels == best {
			countBest++
		}
	}
	if countBest > 1 {
		d.MultipleBest++
	}
	// At most 2^3-1 legal intents. A single ranking exposes the full shared
	// candidate set; equivalent cards in the second deck are interchangeable.
	table := bot.Observe(st, st.Trick.TurnSeat)
	available, err := models[0].Rank(table, 8)
	if err != nil {
		return err
	}
	candidateBest := -4
	faceKey := func(values []int) [54]uint8 {
		var key [54]uint8
		for _, id := range values {
			key[id%54]++
		}
		return key
	}
	for _, action := range available {
		cards, e := ids(action.Action)
		if e != nil {
			return e
		}
		for _, opt := range opts {
			if faceKey(opt.Cards.CardIDs) == faceKey(cards) {
				candidateBest = max(candidateBest, opt.NetLevels)
			}
		}
	}
	if candidateBest == -4 {
		return fmt.Errorf("seed %d turn %d no legal ranked candidate", d.Seed, turn)
	}
	if best == candidateBest {
		d.OptimalCandidateAvailable++
	}
	d.CandidateGapNetLevels += best - candidateBest
	var picked [3][]int
	var regret [3]int
	for i, m := range models {
		cards := basePlay
		if i != 0 {
			cards, err = (policy{m, "raw"}).play(st)
			if err != nil {
				return err
			}
		}
		cards = slices.Clone(cards)
		slices.Sort(cards)
		picked[i] = cards
		found := false
		for _, o := range opts {
			// No more than three cards per seat. Compare entity IDs; the
			// engine legal list can contain both copies of the same face.
			ids := slices.Clone(o.Cards.CardIDs)
			slices.Sort(ids)
			if slices.Equal(ids, cards) {
				regret[i] = best - o.NetLevels
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("seed %d turn %d model %d unrecognized play %v", d.Seed, turn, i, cards)
		}
		d.MissedNetLevels[i] += regret[i]
		if regret[i] == 0 {
			d.Optimal[i]++
		}
	}
	for i := range d.Corrected {
		model := i + 1
		if regret[model] < regret[0] {
			d.LowerRegret[i]++
			if regret[model] == 0 {
				d.Corrected[i]++
			}
		} else if regret[model] > regret[0] {
			d.HigherRegret[i]++
			if regret[0] == 0 {
				d.NewlyMissed[i]++
			}
		}
	}
	if d.Example == nil && (regret[0] != regret[1] || regret[0] != regret[2]) {
		e := diagnosticExample{Seed: d.Seed, PlayIndex: turn, Seat: st.Trick.TurnSeat, AttackerPoints: st.Points, Choices: opts, Picked: picked, Regret: regret, Trump: st.Trump.Trump, Bottom: cardNames(st.Bottom), CallerSeat: st.CallerSeat, BottomOwnerSeat: st.BottomOwnerSeat, LeaderSeat: st.Trick.LeaderSeat}
		for i, s := range st.Seats {
			e.HandNames[i] = cardNames(s.Hand)
			if prior := st.Trick.Plays[i]; prior != nil {
				e.TableCards[i] = cardNames(prior.Cards)
			}
			for _, c := range s.Hand {
				e.Hands[i] = append(e.Hands[i], c.ID)
			}
		}
		d.Example = &e
	}
	return nil
}

type endingComparison struct {
	ModelIndex, Corrected, NewlyMissed, LowerRegret, HigherRegret int
	BaselineOptimalRate, CandidateOptimalRate, RegretReduction    float64
	RegretReduction95, OptimalRateChange95                        [2]float64
}

// Regret is measured per informative decision. Jointly resampling whole deals
// preserves both within-deal correlation and the baseline/candidate pairing.
func compareEndings(deals []diagnosticDeal, total diagnosticCounts) []endingComparison {
	if total.Informative == 0 {
		return nil
	}
	const draws = 10000
	denom := float64(total.Informative)
	comparisons := make([]endingComparison, 2)
	for i := range comparisons {
		model := i + 1
		c := &comparisons[i]
		c.ModelIndex = model
		c.Corrected, c.NewlyMissed = total.Corrected[i], total.NewlyMissed[i]
		c.LowerRegret, c.HigherRegret = total.LowerRegret[i], total.HigherRegret[i]
		c.BaselineOptimalRate = float64(total.Optimal[0]) / denom
		c.CandidateOptimalRate = float64(total.Optimal[model]) / denom
		c.RegretReduction = float64(total.MissedNetLevels[0]-total.MissedNetLevels[model]) / denom
		rng := rand.New(rand.NewSource(1769))
		regrets, rates := make([]float64, 0, draws), make([]float64, 0, draws)
		for range draws {
			var regret, optimal, n int
			for range deals {
				d := &deals[rng.Intn(len(deals))]
				n += d.Informative
				regret += d.MissedNetLevels[0] - d.MissedNetLevels[model]
				optimal += d.Optimal[model] - d.Optimal[0]
			}
			if n > 0 {
				regrets = append(regrets, float64(regret)/float64(n))
				rates = append(rates, float64(optimal)/float64(n))
			}
		}
		slices.Sort(regrets)
		slices.Sort(rates)
		lo, hi := int(.025*float64(len(regrets)-1)), int(.975*float64(len(regrets)-1))
		c.RegretReduction95 = [2]float64{regrets[lo], regrets[hi]}
		c.OptimalRateChange95 = [2]float64{rates[lo], rates[hi]}
	}
	return comparisons
}

func diagnose(args []string) error {
	f := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	base := f.String("baseline", "training/run-01/round-12.json", "v1 reference playing all seats and first evaluated model")
	wide := f.String("wide", "training/architecture/wide/round-12.json", "second evaluated model")
	rich := f.String("rich", "training/architecture/rich/round-12.json", "third evaluated model")
	count := f.Int("deals", 512, "held-out deals")
	workers := f.Int("workers", 4, "concurrent deals")
	start := f.Int64("seed", 23000000, "first independent deal seed")
	out := f.String("out", "training/architecture/endgames.json", "diagnostic report")
	if err := f.Parse(args); err != nil {
		return err
	}
	paths := [3]string{*base, *wide, *rich}
	var hashes [3]string
	var models [3]*bot.LearnModel
	for i, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		hashes[i] = fmt.Sprintf("%x", sha256.Sum256(raw))
		models[i], err = load(p)
		if err != nil {
			return err
		}
	}
	if models[0].Version != 1 {
		return fmt.Errorf("expected a v1 checkpoint for the fixed visiting policy")
	}
	deals, err := parallel(*count, *workers, func(i int) (diagnosticDeal, error) { return evaluateEndings(*start+int64(i), models) })
	if err != nil {
		return err
	}
	var total diagnosticCounts
	var example *diagnosticExample
	represented := 0
	for i := range deals {
		d := &deals[i]
		total.Eligible += d.Eligible
		total.Forced += d.Forced
		total.AllChoicesTie += d.AllChoicesTie
		total.Informative += d.Informative
		total.MultipleBest += d.MultipleBest
		total.OptimalCandidateAvailable += d.OptimalCandidateAvailable
		total.CandidateGapNetLevels += d.CandidateGapNetLevels
		if d.Informative > 0 {
			represented++
		}
		for j := range models {
			total.Optimal[j] += d.Optimal[j]
			total.MissedNetLevels[j] += d.MissedNetLevels[j]
		}
		for j := range total.Corrected {
			total.Corrected[j] += d.Corrected[j]
			total.NewlyMissed[j] += d.NewlyMissed[j]
			total.LowerRegret[j] += d.LowerRegret[j]
			total.HigherRegret[j] += d.HigherRegret[j]
		}
		if example == nil && d.Example != nil {
			example = d.Example
		}
		d.Example = nil
	}
	_, err = save(*out, map[string]any{
		"seed": *start, "deals": *count, "workers": *workers, "cliArgs": args, "modelOrder": paths, "modelSHA256": hashes,
		"visitor":           "greedy raw baseline at all four seats; same positions for every model",
		"target":            "exact team minimax net level difference to round settlement; ties among distinct legal card-ID sets count as optimal",
		"pairedComparisons": compareEndings(deals, total),
		"intervalMethod":    "percentile bootstrap, 10000 resamples of whole deals; regret reduction is baseline minus candidate per informative decision; optimal rate change is candidate minus baseline",
		"informativeDeals":  represented, "summary": total, "byDeal": deals, "firstDifferingRegret": example,
	})
	if err == nil {
		fmt.Printf("informative=%d from %d/%d deals, optimal=%v, summed missed levels=%v\n", total.Informative, represented, *count, total.Optimal, total.MissedNetLevels)
	}
	return err
}
