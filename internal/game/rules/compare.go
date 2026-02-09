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
		return c.Rank.BaseValue()
	}
	if isBigJoker(c) {
		return RVBigJoker
	}
	if isSmallJoker(c) {
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
	return RVTrumpBase + c.Rank.BaseValue()
}

// compareTwoCards 比较两张牌大小（使用时注意先后手顺序对于相等牌力的影响）
func compareTwoCards(a, b Card, t Trump) int {
	return rankValue(a, t) - rankValue(b, t)
}

// isSameBlockType 判断两个牌型是否一致
func isSameBlockType(a, b Block) bool {
	bt0, sc0, tl0, cl0 := a.Type, a.SuitClass, a.TractorLen, len(a.Cards)
	bt1, sc1, tl1, cl1 := b.Type, b.SuitClass, b.TractorLen, len(b.Cards)
	if bt1 != bt0 || sc1 != sc0 || tl1 != tl0 || cl1 != cl0 {
		return false
	}
	return true
}

// CompareTwoBlocks 比较两个牌型大小（使用时注意先后手顺序对于相等牌力的影响）
func CompareTwoBlocks(a, b Block) (int, error) {
	if isSameBlockType(a, b) {
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
		if !isSameBlockType(blocks[0], blocks[i]) {
			return fmt.Errorf("牌型不匹配，不可比较")
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].RankValue > blocks[j].RankValue
	})
	return nil
}
