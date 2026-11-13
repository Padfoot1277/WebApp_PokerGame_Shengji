package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"upgrade-lan/internal/arena"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/endgame"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

type policy struct {
	model *bot.LearnModel
	mode  string
}

const policyHelp = "checkpoint JSON or built-in policy (legacy, rules-endgame, rules [6 cards], rules-8, rules-8x5, rules-HxK with optional -points, -diverse, -both, -tail2, or -tail3)"

var ruleSearches = map[string]bot.RuleSearch{
	"rules":         {MaxHand: 6, Candidates: 3},
	"rules-8":       {MaxHand: 8, Candidates: 3},
	"rules-8x5":     {MaxHand: 8, Candidates: 5},
	"rules-10":      {MaxHand: 10, Candidates: 3},
	"rules-10x5":    {MaxHand: 10, Candidates: 5},
	"rules-endgame": {MaxHand: endgame.MaxHand, Candidates: 3},
}

func lookupRuleSearch(name string) (bot.RuleSearch, bool) {
	if preset, ok := ruleSearches[name]; ok {
		return preset, true
	}
	parts := strings.Split(name, "-")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "rules" {
		return bot.RuleSearch{}, false
	}
	var s bot.RuleSearch
	if _, err := fmt.Sscanf(parts[1], "%dx%d", &s.MaxHand, &s.Candidates); err != nil || fmt.Sprintf("%dx%d", s.MaxHand, s.Candidates) != parts[1] || s.MaxHand < endgame.MaxHand || s.MaxHand > 25 || s.Candidates < 1 {
		return bot.RuleSearch{}, false
	}
	if len(parts) == 3 {
		switch parts[2] {
		case "tail2":
			s.ExactTail = 2
		case "tail3":
			s.ExactTail = 3
		case "points":
			s.PreferPoints = true
		case "diverse":
			s.Diverse = true
		case "both":
			s.PreferPoints, s.Diverse = true, true
		default:
			return bot.RuleSearch{}, false
		}
	}
	return s, true
}

func ruleMode(name string) bool {
	_, ok := lookupRuleSearch(name)
	return name == "legacy" || ok
}

func loadPolicy(name, mode string) (policy, error) {
	if ruleMode(name) {
		return policy{mode: name}, nil
	}
	m, err := load(name)
	return policy{model: m, mode: mode}, err
}

func (p policy) play(st game.GameState) ([]int, error) {
	var a bot.Action
	var err error
	switch p.mode {
	case "late-raw":
		if n := len(st.Seats[st.Trick.TurnSeat].Hand); n >= 4 && n <= 6 {
			return (policy{p.model, "raw"}).play(st)
		}
		return (policy{p.model, "search"}).play(st)
	case "late-roll":
		return lateRollPlay(st, p.model)
	case "midroll":
		return midrollPlay(st, p.model)
	case "hybrid":
		a, err = bot.HybridPlay(st, p.model)
	case "search":
		a, err = bot.LearnedPlay(st, p.model)
	case "raw":
		a, _, err = p.model.Play(bot.Observe(st, st.Trick.TurnSeat), nil)
	case "legacy":
		a, err = bot.DecideFull(bot.Observe(st, st.Trick.TurnSeat), nil)
	default:
		search, ok := lookupRuleSearch(p.mode)
		if !ok {
			return nil, fmt.Errorf("unsupported policy %q", p.mode)
		}
		a, err = search.Play(st)
	}
	if err != nil {
		return nil, err
	}
	return ids(a)
}

type leg struct {
	Team, CallerTeam, NetLevels, AttackerPoints, Moves int
}
type pair struct {
	Seed          int64
	Legs          [2]leg
	MeanNetLevels float64
}
type evaluated struct {
	pair      pair
	replay    arena.Replay
	durations []float64
}

func matchup(seed int64, playModel func(game.GameState) ([]int, error), b policy, record, mixed, endingsOnly, midOnly bool) (evaluated, error) {
	st, err := arena.NewRound(seed)
	if err != nil {
		return evaluated{}, err
	}
	result := evaluated{pair: pair{Seed: seed}}
	for team := range 2 {
		var trace *arena.Replay
		if record && team == 0 {
			result.replay.Seed = seed
			trace = &result.replay
		}
		moves := 0
		final, err := arena.Play(st, func(s game.GameState) ([]int, error) {
			moves++
			// In mixed games the other three seats use the opponent policy.
			// Rotate the evaluated seat between teammates across deals.
			useModel := s.Seats[s.Trick.TurnSeat].Team == team
			if mixed {
				useModel = s.Trick.TurnSeat == team+2*int(uint64(seed)%2)
			}
			if endingsOnly && !endgame.Eligible(s) {
				useModel = false
			}
			if midOnly {
				count := len(s.Seats[s.Trick.TurnSeat].Hand)
				useModel = useModel && count >= 6 && count <= 10
			}
			if useModel {
				start := time.Now()
				cards, e := playModel(s)
				result.durations = append(result.durations, float64(time.Since(start))/float64(time.Millisecond))
				return cards, e
			}
			return b.play(s)
		}, trace)
		if err != nil {
			return result, fmt.Errorf("seed %d team %d: %w", seed, team, err)
		}
		net := arena.Reward(final, team)
		result.pair.Legs[team] = leg{team, st.Seats[st.CallerSeat].Team, net, final.RoundPointsFinal, moves}
		result.pair.MeanNetLevels += float64(net) / 2
	}
	return result, nil
}

type report struct {
	SearchReply, SearchValue                               string `json:",omitempty"`
	LateReference                                          string `json:",omitempty"`
	OpponentTacticsMasked                                  bool
	EndingsOnly                                            bool
	MidOnly                                                bool
	ModelDecisions                                         int
	Mixed                                                  bool
	Model, Mode, Opponent, OpponentMode                    string
	Seed                                                   int64
	Pairs, Games, Wins, Draws, Losses                      int
	NetLevelsPerGame, CallerNetPerGame, AttackerNetPerGame float64
	PairedBootstrap95                                      [2]float64
	DecisionMSMedian, DecisionMS95                         float64
	Deals                                                  []pair
}

func evaluate(args []string) error {
	f := flag.NewFlagSet("eval", flag.ContinueOnError)
	model := f.String("model", "", policyHelp)
	opp := f.String("opponent", "legacy", policyHelp)
	mode := f.String("mode", "search", "search, hybrid, midroll, late-roll, late-raw, late-rank, or raw")
	opMode := f.String("opponent-mode", "search", "search, hybrid, midroll, late-roll, late-raw, or raw for another checkpoint")
	pairs := f.Int("pairs", 32, "number of matched deals")
	workers := f.Int("workers", 4, "concurrent pairs")
	seed := f.Int64("seed", 8500000, "first held-out deal seed")
	out := f.String("out", "", "report file")
	trace := f.String("trace", "", "save first leg of first pair for replay")
	mixed := f.Bool("mixed", false, "replace only one player on the evaluated team")
	endingsOnly := f.Bool("endings-only", false, "use opponent policy on both teams until all hands have at most three cards")
	midOnly := f.Bool("mid-only", false, "use evaluated policy only when its acting seat holds 6 through 10 cards")
	maskTactics := f.Bool("mask-opponent-tactics", false, "drop tactical input columns from the loaded opponent checkpoint for an inference ablation")
	searchReply := f.String("search-reply", "", "override the evaluated search model for simulated replies")
	searchValue := f.String("search-value", "", "override the evaluated search model for next-trick value")
	lateReference := f.String("late-reference", "", "with mode late-rank, use this checkpoint for search outside 4..6 cards and for the fixed top three within that window")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *out == "" || *pairs < 2 {
		return fmt.Errorf("specify -out and at least two pairs")
	}
	if *midOnly && *endingsOnly {
		return fmt.Errorf("choose either -mid-only or -endings-only")
	}
	player, err := loadPolicy(*model, *mode)
	if err != nil {
		return err
	}
	a := player.model
	if a == nil && (*searchReply != "" || *searchValue != "" || *lateReference != "" || *mode == "late-rank") {
		return fmt.Errorf("checkpoint overrides require a learned model")
	}
	b, err := loadPolicy(*opp, *opMode)
	if err != nil {
		return err
	}
	if *maskTactics && b.model == nil {
		return fmt.Errorf("tactics masking requires a tactical opponent checkpoint")
	}
	if *maskTactics {
		b.model, err = b.model.WithoutTacticalInputs()
		if err != nil {
			return err
		}
	}
	playModel := player.play
	if *searchReply != "" || *searchValue != "" {
		if *mode != "search" {
			return fmt.Errorf("search overrides require search mode")
		}
		reply, value := a, a
		if *searchReply != "" {
			reply, err = load(*searchReply)
			if err != nil {
				return err
			}
		}
		if *searchValue != "" {
			value, err = load(*searchValue)
			if err != nil {
				return err
			}
		}
		playModel = func(st game.GameState) ([]int, error) {
			action, err := bot.SearchWithModels(st, a, reply, value)
			if err != nil {
				return nil, err
			}
			return ids(action)
		}
	}
	if *mode == "late-rank" || *lateReference != "" {
		if *mode != "late-rank" || *lateReference == "" {
			return fmt.Errorf("late-rank and -late-reference must be used together")
		}
		ref, e := load(*lateReference)
		if e != nil {
			return e
		}
		playModel = func(st game.GameState) ([]int, error) {
			seat := st.Trick.TurnSeat
			if n := len(st.Seats[seat].Hand); n < 4 || n > 6 {
				return (policy{ref, "search"}).play(st)
			}
			table := bot.Observe(st, seat)
			return rerankLate(table, ref, a)
		}
	}
	games, err := parallel(*pairs, *workers, func(i int) (evaluated, error) {
		return matchup(*seed+int64(i), playModel, b, i == 0 && *trace != "", *mixed, *endingsOnly, *midOnly)
	})
	if err != nil {
		return err
	}
	r := report{Mixed: *mixed, Model: *model, Mode: player.mode, Opponent: *opp, OpponentMode: b.mode, Seed: *seed, Pairs: *pairs, Games: 2 * *pairs}
	r.EndingsOnly = *endingsOnly
	r.SearchReply, r.SearchValue = *searchReply, *searchValue
	r.LateReference = *lateReference
	r.OpponentTacticsMasked = *maskTactics
	r.MidOnly = *midOnly
	var durations []float64
	for _, g := range games {
		r.Deals = append(r.Deals, g.pair)
		durations = append(durations, g.durations...)
		r.NetLevelsPerGame += g.pair.MeanNetLevels / float64(*pairs)
		for _, l := range g.pair.Legs {
			if l.Team == l.CallerTeam {
				r.CallerNetPerGame += float64(l.NetLevels) / float64(*pairs)
			} else {
				r.AttackerNetPerGame += float64(l.NetLevels) / float64(*pairs)
			}
			switch {
			case l.NetLevels > 0:
				r.Wins++
			case l.NetLevels < 0:
				r.Losses++
			default:
				r.Draws++
			}
		}
	}
	// Resample whole deals, retaining the correlation between the swapped legs.
	rng := rand.New(rand.NewSource(441))
	means := make([]float64, 10000)
	for i := range means {
		for range games {
			means[i] += games[rng.Intn(len(games))].pair.MeanNetLevels
		}
		means[i] /= float64(len(games))
	}
	sort.Float64s(means)
	r.PairedBootstrap95 = [2]float64{means[249], means[9749]}
	sort.Float64s(durations)
	r.ModelDecisions = len(durations)
	if len(durations) > 0 {
		r.DecisionMSMedian = durations[(len(durations)-1)/2]
		r.DecisionMS95 = durations[int(math.Ceil(float64(len(durations))*.95))-1]
	}
	if *trace != "" {
		verified, err := games[0].replay.Verify()
		if err != nil {
			return err
		}
		if verified.RoundPointsFinal != games[0].pair.Legs[0].AttackerPoints {
			return fmt.Errorf("replay settlement mismatch")
		}
		if _, err := save(*trace, games[0].replay); err != nil {
			return err
		}
	}
	if _, err = save(*out, r); err != nil {
		return err
	}
	fmt.Printf("%d paired deals, W/D/L %d/%d/%d, levels/game %+.3f (paired 95%% interval %.3f to %.3f), p95 %.1f ms\n", r.Pairs, r.Wins, r.Draws, r.Losses, r.NetLevelsPerGame, r.PairedBootstrap95[0], r.PairedBootstrap95[1], r.DecisionMS95)
	return nil
}

func replay(args []string) error {
	f := flag.NewFlagSet("replay", flag.ContinueOnError)
	path := f.String("file", "", "arena replay")
	at := f.Int("at", -1, "zero-based play index to solve (at most three cards per seat)")
	show := f.Bool("show", false, "print the actual cards, winning seat and running score for every trick")
	if err := f.Parse(args); err != nil {
		return err
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	var r arena.Replay
	if err = json.Unmarshal(data, &r); err != nil {
		return err
	}
	final, err := r.Verify()
	if err != nil {
		return err
	}
	fmt.Printf("seed %d, %d legal plays, attacker points %d, caller +%d, attackers +%d\n", r.Seed, len(r.Moves), final.RoundPointsFinal, final.CallerDelta, final.DefenderDelta)
	if *show {
		st, e := arena.NewRound(r.Seed)
		if e != nil {
			return e
		}
		for _, move := range r.Moves {
			next, e := arena.Step(st, move.CardIDs)
			if e != nil {
				return e
			}
			if next.TrickIndex != st.TrickIndex {
				fmt.Printf("第 %d 墩", next.TrickIndex)
				for i := range 4 {
					seat := (next.Trick.LastLeaderSeat + i) % 4
					fmt.Printf("  %s %s", game.SeatName(seat), cardNames(next.Trick.LastPlays[seat].Cards))
				}
				fmt.Printf("  | %s 赢墩，打家累计 %d 分\n", game.SeatName(next.Trick.WinnerSeat), next.Points)
			}
			st = next
		}
	}
	if *at < 0 {
		return nil
	}
	if *at >= len(r.Moves) {
		return fmt.Errorf("index outside recorded moves")
	}
	st, err := arena.NewRound(r.Seed)
	if err != nil {
		return err
	}
	for _, m := range r.Moves[:*at] {
		st, err = arena.Step(st, m.CardIDs)
		if err != nil {
			return err
		}
	}
	options, err := endgame.Choices(st)
	if err != nil {
		return err
	}
	fmt.Printf("seat %d, recorded IDs %v\n", st.Trick.TurnSeat, r.Moves[*at].CardIDs)
	for _, o := range options {
		fmt.Printf("play %v, guaranteed net levels %+d\n", o.Cards.CardIDs, o.NetLevels)
	}
	return nil
}

func cardNames(cards []rules.Card) string {
	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = string(c.Suit) + string(c.Rank)
		if c.Suit == rules.BigJoker || c.Suit == rules.SmallJoker {
			names[i] = string(c.Rank)
		}
	}
	return strings.Join(names, " ")
}
