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
	// 剩余可用牌池（会被逐步删除）
	remaining := append([]Card(nil), selected...)
	sc := selected[0].SuitClass
	for _, blocks := range bigBlocks {
		for i := 0; i < len(blocks); i++ {
			big := blocks[i]
			// 在剩余牌池里找“同牌型”的候选牌组，按牌力从大到小返回
			cands, err := FindBlocksInHand(remaining, t, sc, big.Type, big.TractorLen)
			if err != nil {
				return false, err
			}
			if len(cands) == 0 {
				return false, nil
			}
			best := cands[0]
			cmp, err := CompareTwoBlocks(big, best)
			if err != nil {
				return false, err
			}
			if cmp >= 0 {
				return false, nil
			}
			remaining = deleteCards(remaining, best.Cards)
		}
	}
	return true, nil
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
