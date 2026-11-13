package bot

import (
	"fmt"

	"upgrade-lan/internal/game/rules"
)

const tacticalExtraSize = 14

// Copy all common weights, including learned biases. Additional columns are zero
// on expansion. Dropping those columns reproduces an inference run with just
// the extra inputs masked, while preserving training's changes to common weights.
func copyTacticalColumns(src, dst *LearnModel) {
	a, b := src.shape(), dst.shape()
	dst.Weights = make([]float64, b.parameters)
	copy(dst.Weights[:b.actionWeights], src.Weights[:a.actionWeights])
	for j := range a.width {
		from, to := a.actionWeights+j*a.moveSize, b.actionWeights+j*b.moveSize
		w := min(a.moveSize, b.moveSize)
		copy(dst.Weights[to:to+w], src.Weights[from:from+w])
	}
	copy(dst.Weights[b.actionBias:b.policySkip], src.Weights[a.actionBias:a.policySkip])
	copy(dst.Weights[b.policySkip:], src.Weights[a.policySkip:])
}

func ExpandTactical(base *LearnModel) (*LearnModel, error) {
	if err := base.Validate(); err != nil {
		return nil, err
	}
	if base.Version != 1 {
		return nil, fmt.Errorf("tactical expansion requires a v1 checkpoint")
	}
	m := &LearnModel{Version: 2, Variant: "tactical"}
	copyTacticalColumns(base, m)
	return m, nil
}

func (m *LearnModel) WithoutTacticalInputs() (*LearnModel, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if m.Variant != "tactical" {
		return nil, fmt.Errorf("expected tactical checkpoint")
	}
	dst := &LearnModel{Version: 1}
	copyTacticalColumns(m, dst)
	return dst, nil
}

type tacticalBlockKey struct {
	seat   int
	suit   rules.SuitClass
	kind   rules.BlockType
	length int
}
type tacticalContext struct {
	bySuit [4]map[rules.SuitClass][]rules.Card
	blocks map[tacticalBlockKey][]rules.Block
}

func prepareTactics(t Table) tacticalContext {
	ctx := tacticalContext{blocks: make(map[tacticalBlockKey][]rules.Block)}
	for seat, hand := range t.Hands {
		ctx.bySuit[seat] = make(map[rules.SuitClass][]rules.Card, 5)
		for _, c := range hand {
			ctx.bySuit[seat][c.SuitClass] = append(ctx.bySuit[seat][c.SuitClass], c)
		}
	}
	return ctx
}

// Per pending seat: can it legally beat the incumbent after our proposed play?
// These are independent options, rather than a prediction that a player will
// take the lead or that later seats will overtake the same way.
func (ctx *tacticalContext) encode(t *Table, cards []rules.Card, f []float64) {
	v := &t.View
	actor := v.MySeat
	lead := v.Trick.Plays[v.Trick.LeaderSeat]
	leaderSuit := cards[0].SuitClass
	big := actor
	var shape, incumbent [][]rules.Block
	if lead == nil {
		shape, _ = rules.DecomposeThrow(cards, v.Trump.Trump, leaderSuit)
		incumbent = shape
	} else {
		shape = lead.Blocks
		leaderSuit = lead.SuitClass
		big = v.Trick.BiggerSeat
		if big < 0 || big >= 4 || v.Trick.Plays[big] == nil {
			return
		}
		incumbent = v.Trick.Plays[big].Blocks
		if _, ok, _ := rules.ClassifyPaddingAndComparable(cards, leaderSuit); ok {
			if blocks, win, _ := rules.WinningBlocks(incumbent, cards, v.Trump.Trump); win {
				incumbent, big = blocks, actor
			}
		}
	}
	if len(shape) == 0 || len(incumbent) == 0 {
		return
	}
	if len(shape) == 1 && len(shape[0]) == 1 {
		switch shape[0][0].Type {
		case rules.BlockSingle:
			f[0] = 1
		case rules.BlockPair:
			f[1] = 1
		case rules.BlockTractor:
			f[2] = 1
		}
	} else {
		f[3] = 1
	}
	f[4+(big-actor+4)%4] = 1
	for off := 1; off < 4; off++ {
		seat := (actor + off) % 4
		if v.Trick.Plays[seat] != nil {
			continue
		}
		win, unknown := ctx.canCover(t, seat, len(cards), leaderSuit, shape, incumbent)
		f[7+off] = flag(win)
		f[10+off] = flag(unknown)
	}
}

func (ctx *tacticalContext) canCover(t *Table, seat, n int, suit rules.SuitClass, shape, incumbent [][]rules.Block) (bool, bool) {
	// A partly depleted lead suit forces mixed padding. A void player may
	// win only by ruffing with the entire required structure.
	pool := ctx.bySuit[seat][suit]
	if len(pool) == 0 {
		suit = rules.SCTrump
		pool = ctx.bySuit[seat][suit]
	}
	if len(pool) < n {
		return false, false
	}
	trump := t.View.Trump.Trump
	if len(incumbent) == 1 && len(incumbent[0]) == 1 {
		target := incumbent[0][0]
		key := tacticalBlockKey{seat, suit, target.Type, target.TractorLen}
		blocks, exists := ctx.blocks[key]
		if !exists {
			blocks, _ = rules.FindBlocksInHand(pool, trump, suit, target.Type, target.TractorLen)
			ctx.blocks[key] = blocks
		}
		// A complete block matching a single lead structure automatically
		// satisfies follow obligations once the required suit is honored.
		if len(blocks) == 0 {
			return false, false
		}
		order, err := rules.CompareTwoBlocks(target, blocks[0])
		return err == nil && order < 0, false
	}
	// Explore a bounded number of same-suit subsets. On budget exhaustion,
	// report unknown rather than treating the play as a guaranteed winner.
	if n > 5 {
		return false, true
	}
	const budget = 16
	attempts := 0
	indices := make([]int, n)
	selected := make([]rules.Card, n)
	for i := range indices {
		indices[i] = i
	}
	for {
		for i, j := range indices {
			selected[i] = pool[j]
		}
		if win, _ := rules.CompareForTrickWin(incumbent, selected, trump); win && rules.ValidateHideCheck(t.Hands[seat], selected, shape, trump) == nil {
			return true, false
		}
		j := n - 1
		for j >= 0 && indices[j] == len(pool)-n+j {
			j--
		}
		if j < 0 {
			return false, false
		}
		attempts++
		if attempts >= budget {
			return false, true
		}
		indices[j]++
		for k := j + 1; k < n; k++ {
			indices[k] = indices[k-1] + 1
		}
	}
}
