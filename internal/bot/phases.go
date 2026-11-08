package bot

import (
	"fmt"
	"sort"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

type Action struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

func handOf(s game.ViewState) []rules.Card {
	h := []rules.Card{}
	for _, g := range s.MyHand {
		h = append(h, g...)
	}
	sort.Slice(h, func(i, j int) bool { return h[i].ID < h[j].ID })
	return h
}
func classified(h []rules.Card, t rules.Trump) []rules.Card {
	out := append([]rules.Card(nil), h...)
	for i := range out {
		out[i].SuitClass = rules.ComputeSuitClass(out[i], t)
	}
	return out
}

// Evaluate retained control, intact combinations and opportunities to ruff a void suit.
func handValue(h []rules.Card, t rules.Trump) float64 {
	h = classified(h, t)
	score := 0.0
	counts := map[rules.SuitClass]int{}
	for _, c := range h {
		counts[c.SuitClass]++
		v := strength(c, t)
		score += v * 0.4
		if c.SuitClass == rules.SCTrump {
			score += 5
		}
		if c.Rank == rules.RA {
			score += 5
		}
	}
	for _, sc := range []rules.SuitClass{rules.SCH, rules.SCS, rules.SCD, rules.SCC, rules.SCTrump} {
		pairs, _ := rules.FindBlocksInHand(h, t, sc, rules.BlockPair, 0)
		score += float64(len(pairs)) * 8
		tractors, _ := rules.FindBlocksInHand(h, t, sc, rules.BlockTractor, 2)
		score += float64(len(tractors)) * 10
		if sc != rules.SCTrump && counts[sc] == 0 && counts[rules.SCTrump] > 0 {
			score += 6
		}
	}
	return score
}
func phaseAction(s game.ViewState) (Action, error) {
	if s.MySeat < 0 || s.MySeat >= 4 {
		return Action{}, fmt.Errorf("invalid seat")
	}
	h := handOf(s)
	t := s.Trump.Trump
	if s.Phase == game.PhaseBottom {
		ids, err := bottom(h, t)
		return Action{"game.put_bottom", game.PutBottomPayload{DiscardIDs: ids}}, err
	}
	pass := Action{"game.call_pass", struct{}{}}
	if s.Phase != game.PhaseCallTrump && s.Phase != game.PhaseTrumpFight {
		return Action{}, fmt.Errorf("unsupported policy phase")
	}
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
	level := t.LevelRank
	if calling {
		level = s.Teams[s.Seats[s.MySeat].Team].LevelRank
	}
	current := handValue(h, t)
	best := pass
	bestScore := 0.0
	ally := s.BottomOwnerSeat >= 0 && s.BottomOwnerSeat < 4 && s.Seats[s.BottomOwnerSeat].Team == s.Seats[s.MySeat].Team
	for _, j := range available {
		if !rules.IsBigJoker(j) && !rules.IsSmallJoker(j) {
			continue
		}
		for _, suit := range []rules.Suit{rules.Heart, rules.Diamond, rules.Spade, rules.Club} {
			levels := []rules.Card{}
			for _, c := range available {
				if c.Suit == suit && c.Rank == level {
					levels = append(levels, c)
				}
			}
			if len(levels) == 0 {
				continue
			}
			if len(levels) > 2 {
				levels = levels[:2]
			}
			candidate := rules.Trump{HasTrumpSuit: true, Suit: suit, LevelRank: level}
			score := 0.0
			var a Action
			if calling {
				_, locked, err := rules.ValidateCallTrump(level, j, levels)
				if err != nil {
					continue
				}
				// Compare against hard trump; require a meaningful suit holding to take the bottom.
				score = handValue(h, candidate) - handValue(h, rules.Trump{LevelRank: level}) - 16
				if locked {
					score += 5
				}
				ids := []int{}
				for _, c := range levels {
					ids = append(ids, c.ID)
				}
				a = Action{"game.call_trump", game.CallTrumpPayload{JokerID: j.ID, LevelIDs: ids}}
			} else {
				if s.Trump.Locked || len(levels) != 2 || suit == t.Suit {
					continue
				}
				if _, err := rules.ValidateChangeTrump(level, j, levels[0], levels[1]); err != nil {
					continue
				}
				score = handValue(h, candidate) - current - 8
				if ally {
					score -= 14
				}
				a = Action{"game.change_trump", game.ChangeTrumpPayload{JokerID: j.ID, LevelIDs: []int{levels[0].ID, levels[1].ID}}}
			}
			if score > bestScore {
				best, bestScore = a, score
			}
		}
	}
	if !calling && t.HasTrumpSuit {
		for i, j := range available {
			for _, k := range available[i+1:] {
				if rules.ValidateAttackTrump(j, k) != nil {
					continue
				}
				hard := rules.Trump{LevelRank: level}
				score := handValue(h, hard) - current - 8
				// Taking the bottom away from an opponent earns a small control bonus.
				if s.Trump.CallerSeat >= 0 && s.Seats[s.Trump.CallerSeat].Team != s.Seats[s.MySeat].Team {
					score += 12
				}
				if ally {
					score -= 14
				}
				if score > bestScore {
					bestScore = score
					best = Action{"game.attack_trump", game.AttackTrumpPayload{JokerIDs: []int{j.ID, k.ID}}}
				}
			}
		}
	}
	return best, nil
}

// Beam search considers combinations of eight discards, including clearing short suits.
func bottom(h []rules.Card, t rules.Trump) ([]int, error) {
	if len(h) != 33 {
		return nil, fmt.Errorf("bottom requires 33 cards, got %d", len(h))
	}
	type node struct {
		ids   []int
		next  int
		score float64
	}
	beam := []node{{}}
	for depth := 0; depth < 8; depth++ {
		next := []node{}
		for _, b := range beam {
			for i := b.next; i <= len(h)-(8-depth); i++ {
				ids := append(append([]int(nil), b.ids...), h[i].ID)
				removed := map[int]bool{}
				for _, id := range ids {
					removed[id] = true
				}
				keep := []rules.Card{}
				penalty := 0.0
				for _, c := range h {
					if removed[c.ID] {
						penalty += points(c) * 1.5
					} else {
						keep = append(keep, c)
					}
				}
				next = append(next, node{ids: ids, next: i + 1, score: handValue(keep, t) - penalty})
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return next[i].score > next[j].score })
		if len(next) > 96 {
			next = next[:96]
		}
		beam = next
	}
	if len(beam) == 0 {
		return nil, fmt.Errorf("no bottom candidate")
	}
	return beam[0].ids, nil
}
