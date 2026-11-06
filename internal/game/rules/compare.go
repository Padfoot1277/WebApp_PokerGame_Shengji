package rules

import (
	"fmt"
	"sort"
)

const (
	RVBigJoker   int = 10000
	RVSmallJoker int = 9000
	RVTrumpLevel int = 8500
	RVSubLevel   int = 8200
	RVLevel      int = 8000
	RVTrumpBase  int = 7000
)

// rankValue 返回“同 SuitClass 内”单张牌的强度（越大越强）
func rankValue(c Card, t Trump) int {
	if !isTrumpCard(c, t) {
		return ordinaryRank(c, t)
	}
	if IsBigJoker(c) {
		return RVBigJoker
	}
	if IsSmallJoker(c) {
		return RVSmallJoker
	}
	if c.Rank == t.LevelRank {
		if !t.HasTrumpSuit {
			return RVLevel
		}
		if c.Suit == t.Suit {
			return RVTrumpLevel
		}
		return RVSubLevel
	}
	return RVTrumpBase + ordinaryRank(c, t)
}

// compareTwoCards 比较两张牌大小（使用时注意先后手顺序对于相等牌力的影响）
func compareTwoCards(a, b Card, t Trump) int {
	return rankValue(a, t) - rankValue(b, t)
}

// isBlockTypeComparable 判断两个牌型是否一致
func isBlockTypeComparable(a, b Block) bool {
	bt0, sc0, tl0, cl0 := a.Type, a.SuitClass, a.TractorLen, len(a.Cards)
	bt1, sc1, tl1, cl1 := b.Type, b.SuitClass, b.TractorLen, len(b.Cards)
	if bt1 != bt0 || tl1 != tl0 || cl1 != cl0 {
		return false
	}
	// 若牌域不同且都不是主牌，则不可比较
	if sc1 != sc0 && sc1 != SCTrump && sc0 != SCTrump {
		return false
	}
	return true
}

// CompareTwoBlocks 比较两个牌型大小（使用时注意先后手顺序对于相等牌力的影响）
func CompareTwoBlocks(a, b Block) (int, error) {
	if isBlockTypeComparable(a, b) {
		return a.RankValue - b.RankValue, nil
	}
	return 0, fmt.Errorf("将两个不同牌型的进行比较：%+v， %+v", a, b)
}

// SortBlocksByRank 原地排序：按 Block.RankValue 从大到小
// 只用于“可比”的 blocks（同 SuitClass、同 Type、tractorLen 相同）
func SortBlocksByRank(blocks []Block) error {
	if len(blocks) <= 1 {
		return nil
	}
	for i := 1; i < len(blocks); i++ {
		if !isBlockTypeComparable(blocks[0], blocks[i]) {
			return fmt.Errorf("牌型不匹配，不可比较")
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].RankValue > blocks[j].RankValue
	})
	return nil
}

// CompareForTrickWin 根据bigBlocks里的每一个牌组，找selected中最大的同牌型牌组进行比较；
// 一旦跟牌无法大过当前big方，就返回win=false；如果大于，删除这个牌组，继续下一个牌组的查找和比较；如果全部都大于，则返回win=true。
func CompareForTrickWin(bigBlocks [][]Block, selected []Card, t Trump) (bool, error) {
	_, win, err := WinningBlocks(bigBlocks, selected, t)
	return win, err
}

// Preserve the lead structure when assigning a winning move. Adjacent pairs in
// a follower's hand must not change two required pairs into a tractor for the next player.
func WinningBlocks(bigBlocks [][]Block, selected []Card, t Trump) ([][]Block, bool, error) {
	if len(selected) == 0 {
		return nil, false, nil
	}
	sc, ok := ComputeSuitClassAllSame(selected)
	if !ok {
		return nil, false, nil
	}
	shape := []Block{}
	for _, group := range bigBlocks {
		shape = append(shape, group...)
	}
	need := 0
	for _, block := range shape {
		need += len(block.Cards)
	}
	if need != len(selected) {
		return nil, false, nil
	}
	var assign func(int, []Card) ([]Block, bool, error)
	assign = func(index int, remaining []Card) ([]Block, bool, error) {
		if index == len(shape) {
			return []Block{}, true, nil
		}
		target := shape[index]
		candidates, err := FindBlocksInHand(remaining, t, sc, target.Type, target.TractorLen)
		if err != nil {
			return nil, false, err
		}
		for _, candidate := range candidates {
			cmp, err := CompareTwoBlocks(target, candidate)
			if err != nil {
				return nil, false, err
			}
			if cmp >= 0 {
				continue
			}
			rest, ok, err := assign(index+1, deleteCards(remaining, candidate.Cards))
			if err != nil {
				return nil, false, err
			}
			if ok {
				return append([]Block{candidate}, rest...), true, nil
			}
		}
		return nil, false, nil
	}
	matched, win, err := assign(0, selected)
	if !win || err != nil {
		return nil, false, err
	}
	out := make([][]Block, len(bigBlocks))
	offset := 0
	for i, group := range bigBlocks {
		out[i] = append([]Block(nil), matched[offset:offset+len(group)]...)
		offset += len(group)
	}
	return out, true, nil
}

func deleteCards(hand, delete []Card) []Card {
	if len(delete) == 0 {
		return hand
	}
	rm := make(map[int]struct{}, len(delete))
	for _, card := range delete {
		rm[card.ID] = struct{}{}
	}
	keep := make([]Card, 0, len(hand)-len(delete))
	for _, c := range hand {
		if _, ok := rm[c.ID]; ok {
			continue
		}
		keep = append(keep, c)
	}
	return keep
}

func ordinaryRank(c Card, t Trump) int {
	v := c.Rank.BaseValue()
	if c.Rank != t.LevelRank && v > t.LevelRank.BaseValue() {
		v--
	}
	return v
}
