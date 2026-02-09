package rules

import (
	"fmt"
	"sort"
)

type Trump struct {
	HasTrumpSuit bool `json:"hasTrumpSuit"`
	Suit         Suit `json:"suit,omitempty"` // HasTrumpSuit=true 时有效
	LevelRank    Rank `json:"levelRank"`      // 本小局最终级牌（=坐家级数）；硬主时也可填入“本小局先手所在组级牌”作为展示用
}

// BlockType 基础牌型类别
type BlockType string

const (
	BlockSingle  BlockType = "single"
	BlockPair    BlockType = "pair"
	BlockTractor BlockType = "tractor"
)

// Block 基础牌型
type Block struct {
	Type       BlockType `json:"type"`
	SuitClass  SuitClass `json:"suitClass"`
	RankValue  int       `json:"ranks"`       // single/pair: 取原值 ; tractor: 取最高值
	TractorLen int       `json:"tractor_len"` // 拖拉机长度
	Cards      []Card    `json:"cards"`       // 实际使用的牌
}

func isRedSuit(s Suit) bool {
	return s == Heart || s == Diamond
}
func isBlackSuit(s Suit) bool {
	return s == Spade || s == Club
}
func isBigJoker(c Card) bool {
	return c.Suit == BigJoker
}
func isSmallJoker(c Card) bool {
	return c.Suit == SmallJoker
}
func isNormal(c Card) bool {
	return c.Suit != SmallJoker && c.Suit != BigJoker
}

// isTrumpCard：根据 trump 信息判断某张牌是否为主牌
func isTrumpCard(c Card, t Trump) bool {
	if !isNormal(c) {
		return true
	}
	if c.Rank == t.LevelRank {
		return true
	}
	if t.HasTrumpSuit && c.Suit == t.Suit {
		return true
	}
	return false
}

// ComputeSuitClass 主牌归 SCTrump，非主牌归具体花色
func ComputeSuitClass(c Card, t Trump) SuitClass {
	if isTrumpCard(c, t) {
		return SCTrump
	}
	switch c.Suit {
	case Heart:
		return SCH
	case Spade:
		return SCS
	case Diamond:
		return SCD
	case Club:
		return SCC
	default:
		return SCUnknown
	}
}

// ComputeSuitClassAllSame 如果所有牌都属于同一个 SuitClass，则返回该 SuitClass 和 true，用于先手出牌校验/跟牌可比性判断
func ComputeSuitClassAllSame(selected []Card) (SuitClass, bool) {
	if len(selected) == 0 {
		return SCUnknown, false
	}
	sc0 := selected[0].SuitClass
	for _, s := range selected[1:] {
		if s.SuitClass != sc0 {
			return SCMix, false
		}
	}
	return sc0, true
}

// FindBlocksInHand 在手牌中寻找某一牌型并排序
func FindBlocksInHand(hand []Card, t Trump, suitClass SuitClass, bt BlockType, tractorLen int) ([]Block, error) {
	// 筛选牌域
	cards := filterBySuitClass(hand, suitClass)
	// 寻找牌型
	blocks := make([]Block, 0, len(cards))
	switch bt {
	case BlockSingle:
		for _, c := range cards {
			blocks = append(blocks, Block{
				Type:       BlockSingle,
				SuitClass:  suitClass,
				RankValue:  rankValue(c, t),
				TractorLen: 0,
				Cards:      []Card{c},
			})
		}
		// 检查blocks合法性，并降序排序
		if err := SortBlocksByRank(blocks); err != nil {
			return nil, err
		}
		return blocks, nil
	case BlockPair:
		return buildPairs(cards, t, suitClass)
	case BlockTractor:
		if tractorLen < 2 {
			return nil, fmt.Errorf("要查找的牌型非法，拖拉机长度应大于1")
		}
		return buildTractors(cards, t, suitClass, tractorLen)
	default:
		return nil, fmt.Errorf("要查找的牌型非法")
	}
}

// filterBySuitClass 获取手牌中某个牌域的所有牌
func filterBySuitClass(hand []Card, suitClass SuitClass) []Card {
	cards := make([]Card, 0, len(hand))
	for _, c := range hand {
		if c.SuitClass == suitClass {
			cards = append(cards, c)
		}
	}
	return cards
}

// buildPairs 在同牌域中寻找对子（同名牌对：ID 相差 54）
func buildPairs(cards []Card, t Trump, suitClass SuitClass) ([]Block, error) {
	blocks := make([]Block, 0)
	idMap := make(map[int]Card, len(cards))
	for _, c := range cards {
		idMap[c.ID] = c
	}
	for _, c := range cards {
		otherID := c.ID + 54
		other, ok := idMap[otherID]
		if !ok {
			continue
		}
		pair := []Card{c, other}
		blocks = append(blocks, Block{
			Type:       BlockPair,
			SuitClass:  suitClass,
			RankValue:  rankValue(c, t),
			TractorLen: 0,
			Cards:      pair,
		})
	}
	// 降序排序
	if err := SortBlocksByRank(blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

// buildTractors 在同牌域中寻找固定长度的拖拉机
func buildTractors(cards []Card, t Trump, suitClass SuitClass, tractorLen int) ([]Block, error) {
	if tractorLen < 2 {
		return nil, fmt.Errorf("拖拉机长度不可小于2")
	}
	// 先得到所有对子并排序
	pairs, err := buildPairs(cards, t, suitClass)
	if len(pairs) == 0 || err != nil {
		return nil, err
	}
	// 找长度为 tractorLen 的连续下降窗口
	blocks := make([]Block, 0)
	for i := 0; i < len(pairs) && i+tractorLen <= len(pairs); i++ {
		start := pairs[i].RankValue
		ok := true
		cardsUsed := make([]Card, 0, 2*tractorLen)
		for k := 0; k < tractorLen; k++ {
			if pairs[i+k].RankValue != start-k {
				ok = false
				break
			}
			cardsUsed = append(cardsUsed, pairs[i+k].Cards...)
		}
		if ok {
			blocks = append(blocks, Block{
				Type:       BlockTractor,
				SuitClass:  suitClass,
				RankValue:  start,      // 拖拉机最高rank作为值
				TractorLen: tractorLen, // 指定长度
				Cards:      cardsUsed,
			})
		}
	}
	// 降序排序
	if err := SortBlocksByRank(blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

// DecomposeThrow 将同 SuitClass 的甩牌拆成若干牌型组（[][]Block）
// 每个组内 blocks 为同类型牌型，并按 Rank 降序
func DecomposeThrow(selected []Card, t Trump, suitClass SuitClass) ([][]Block, error) {
	if len(selected) < 1 {
		return nil, fmt.Errorf("没有打出牌")
	}
	// 构造所有对子资源
	allPairs, err := buildPairs(selected, t, suitClass)
	if err != nil {
		return nil, err
	}
	// 贪心获取拖拉机，否则视为单对（贪心保证了新构造的[]Block也是降序）
	allUsedCards := make([]Card, 0, len(selected))
	tractorGroups := map[int][]Block{} // tractorLen -> []Block
	pairBlocks := make([]Block, 0)

	for i := 0; i < len(allPairs); {
		startRv := allPairs[i].RankValue
		usedCards := make([]Card, 0)

		step := 0
		for ; i+step < len(allPairs); step++ {
			if allPairs[i+step].RankValue != startRv-step {
				break
			}
			usedCards = append(usedCards, allPairs[i+step].Cards...)
			allUsedCards = append(allUsedCards, allPairs[i+step].Cards...)
		}
		// - 要么 break（遇到不连续）
		// - 要么 step 把尾巴吃完了（i+step == len(allPairs)）
		if step == 1 {
			pairBlocks = append(pairBlocks, allPairs[i])
		} else {
			tractorGroups[step] = append(tractorGroups[step], Block{
				Type:       BlockTractor,
				SuitClass:  suitClass,
				RankValue:  startRv,
				TractorLen: step,
				Cards:      usedCards,
			})
		}
		i += step // 无论哪种情况都要推进 i，避免死循环
	}

	// 从selected中扣除所有已用牌（拖拉机+对子）,得到剩余单张
	usedMap := make(map[int]struct{}, len(allUsedCards))
	for _, card := range allUsedCards {
		usedMap[card.ID] = struct{}{}
	}
	singleBlocks := make([]Block, 0)
	for _, card := range selected {
		if _, exists := usedMap[card.ID]; !exists {
			singleBlocks = append(singleBlocks, Block{
				Type:       BlockSingle,
				SuitClass:  suitClass,
				RankValue:  rankValue(card, t),
				TractorLen: 0,
				Cards:      []Card{card},
			})
		}
	}
	if err = SortBlocksByRank(singleBlocks); err != nil {
		return nil, err
	}

	// 拼装输出：tractor(len大->小) + pairs + singles
	out := make([][]Block, 0, 6)
	if len(tractorGroups) > 0 {
		lens := make([]int, 0, len(tractorGroups))
		for l := range tractorGroups {
			lens = append(lens, l)
		}
		sort.Slice(lens, func(i, j int) bool { return lens[i] > lens[j] })
		for _, l := range lens {
			bs := tractorGroups[l]
			out = append(out, bs)
		}
	}
	if len(pairBlocks) > 0 {
		out = append(out, pairBlocks)
	}
	if len(singleBlocks) > 0 {
		out = append(out, singleBlocks)
	}
	return out, nil
}
