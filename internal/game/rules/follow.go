package rules

import (
	"fmt"
	"sort"
)

// ============================
// 藏牌校验（严格按 A/B/C/D，无 DFS）
// ============================

// ValidateHideCheck 只做“藏牌/跟牌合法性”校验（不做垫牌、比较大小、结算）。
// A. 若 len(handLead) <= N：必须把该牌域手牌全打出（played 中 leadSC 数量 == len(handLead)）
// B. 若 len(handLead) >  N：必须纯域出牌（不混牌域，且域==leadSC）
// C. 纯域下：按 lead blocks 逐块匹配；手里有该型但 played 里没有任何一个候选块 -> 藏牌；否则按降级计划继续
// D. 本次出的牌（纯域）必须全部被 C 步消费完（playSet 删空）
func ValidateHideCheck(hand []Card, played []Card, leadMove [][]Block, trump Trump) error {
	// 本墩每家必须出够先手出的张数
	leadCardNum := countBlocksCards(leadMove)
	if len(played) != leadCardNum {
		return fmt.Errorf("出牌数不合法，打出%d张，需要%d张", len(played), leadCardNum)
	}
	if len(leadMove) == 0 || len(leadMove[0]) == 0 {
		return fmt.Errorf("藏牌校验出错，leadMove出现空组")
	}
	leadSC := leadMove[0][0].SuitClass
	handLead := filterBySuitClass(hand, leadSC)
	// A. handLead <= N：必须全跟域内牌（数量约束）
	if len(handLead) <= leadCardNum {
		cnt := countBySuitClass(played, leadSC)
		if cnt != len(handLead) {
			return fmt.Errorf("不可藏牌, 需打出%d张%s", len(handLead), leadSC)
		}
		return nil
	}
	// B. handLead > N：必须纯域出牌（域一致约束）
	sc, allSame := ComputeSuitClassAllSame(played)
	if !allSame || sc != leadSC {
		return fmt.Errorf("不可藏牌, 需打出%d张%s", leadCardNum, leadSC)
	}

	// C. 逐块匹配 + 降级 + 删除
	handSet := newMultiSet()
	playSet := newMultiSet()
	id2card := make(map[int]Card, len(handLead))
	for _, c := range handLead {
		id2card[c.ID] = c
		handSet.Add(c.ID, 1)
	}
	for _, c := range played {
		playSet.Add(c.ID, 1)
	}
	reqs, err := flattenLeadReqs(leadMove)
	if err != nil {
		return err
	}
	// reqs包含先手每个block的牌组类型，plans包含每个牌组类型的可行方案（含降级），sub表示每个plan里所需的具体子牌组（如拖拉机可拆分为多个对子）
	for _, req := range reqs {
		plans := buildPlans(req)
		matchedReq := false
		for _, plan := range plans {
			hs := handSet.Clone()
			ps := playSet.Clone()
			okPlan := true
			for _, sub := range plan {
				// 判断是否满足SubReq，同时修改hs，ps
				ok, hardFail, e := matchOneSubReq(hs, ps, id2card, trump, leadSC, sub)
				if e != nil {
					return e
				}
				if !ok {
					okPlan = false
					// hardFail：手里有该（子）牌型，但你出的牌里没有任何一个候选块 -> 直接藏牌
					if hardFail {
						return fmt.Errorf("不可藏牌, 拥有%s但没有出牌", sub.String())
					}
					// softFail：手里没有该（子）牌型 -> 尝试下一个降级 plan
					break
				}
			}
			// 如果找到该plan的对应牌，则删除并标记满足
			if okPlan {
				handSet = hs
				playSet = ps
				matchedReq = true
				break
			}
		}
		if !matchedReq {
			return fmt.Errorf("藏牌校验出错，无法满足要求%+v", req)
		}
	}
	// D. playSet 应删空：保证所有出牌都能被“解释/消费”
	if playSet.Total() != 0 {
		return fmt.Errorf("藏牌校验出错，还有%d张牌没被解释", playSet.Total())
	}
	return nil
}

// ============================
// Req/Plan/SubReq
// ============================

type reqBlock struct {
	bt         BlockType
	tractorLen int
	needCards  int
}

type subReq struct {
	bt         BlockType
	tractorLen int
	needCards  int
}

func (s subReq) String() string {
	return fmt.Sprintf("subReq{bt=%s,len=%d,need=%d}", s.bt, s.tractorLen, s.needCards)
}

// flattenLeadReqs 把 Blocks 扁平化为 req 列表
func flattenLeadReqs(groups [][]Block) ([]reqBlock, error) {
	out := make([]reqBlock, 0, 16)
	for _, g := range groups {
		for _, b := range g {
			need := len(b.Cards)
			// 兜底：按类型推
			if need <= 0 {
				switch b.Type {
				case BlockSingle:
					need = 1
				case BlockPair:
					need = 2
				case BlockTractor:
					need = 2 * b.TractorLen
				default:
					return nil, fmt.Errorf("未知的牌组类型: %s", b.Type)
				}
			}
			out = append(out, reqBlock{
				bt:         b.Type,
				tractorLen: b.TractorLen,
				needCards:  need,
			})
		}
	}
	return out, nil
}

// buildPlans 藏牌查验方案（含降级），为了避免递归消耗，直接写死每个情况的返回值。极端情况（如4连拖拉机）只做简单处理。
// - Tractor(L长度)：优先 Tractor；否则，递归消解拖拉机和对子
// - Pair(2)：优先 Pair；否则 Single×2
// - Single(1)：仅 Single
func buildPlans(r reqBlock) [][]subReq {
	switch r.bt {
	case BlockSingle:
		return [][]subReq{{{bt: BlockSingle, needCards: 1}}}
	case BlockPair:
		return [][]subReq{
			{{bt: BlockPair, needCards: 2}},
			{
				{bt: BlockSingle, needCards: 1},
				{bt: BlockSingle, needCards: 1},
			},
		}
	case BlockTractor:
		if r.tractorLen == 2 {
			return [][]subReq{
				{{bt: BlockTractor, tractorLen: 2, needCards: 4}},
				{{bt: BlockPair, needCards: 2}, {bt: BlockPair, needCards: 2}},
				{{bt: BlockPair, needCards: 2}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}},
				{{bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}},
			}
		} else if r.tractorLen == 3 {
			return [][]subReq{
				{{bt: BlockTractor, tractorLen: 3, needCards: 6}},
				{{bt: BlockTractor, tractorLen: 2, needCards: 4}, {bt: BlockPair, needCards: 2}},
				{{bt: BlockTractor, tractorLen: 2, needCards: 4}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}},
				{{bt: BlockPair, needCards: 2}, {bt: BlockPair, needCards: 2}, {bt: BlockPair, needCards: 2}},
				{{bt: BlockPair, needCards: 2}, {bt: BlockPair, needCards: 2}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}},
				{{bt: BlockPair, needCards: 2}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}},
				{{bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}, {bt: BlockSingle, needCards: 1}},
			}
		} else {
			// 对于超级拖拉机，可以做简单处理
			need := r.needCards
			plan1 := make([]subReq, 0, need-1)
			plan1 = append(plan1, subReq{bt: BlockPair, needCards: 2})
			for i := 0; i < need-2; i++ {
				plan1 = append(plan1, subReq{bt: BlockSingle, needCards: 1})
			}
			plan2 := make([]subReq, 0, need)
			for i := 0; i < r.needCards; i++ {
				plan2 = append(plan2, subReq{bt: BlockSingle, needCards: 1})
			}
			return [][]subReq{plan1, plan2}
		}
	default:
		return [][]subReq{{{bt: r.bt, tractorLen: r.tractorLen, needCards: r.needCards}}}
	}
}

// matchOneSubReq：
// - hand 中不存在该子牌型：softFail（ok=false, hardFail=false）
// - hand 中存在该子牌型，但 played 中没有任何一个候选块：hardFail（ok=false, hardFail=true）
// - 找到：删 handSet/playSet 并返回 ok=true
func matchOneSubReq(
	handSet *multiSet,
	playSet *multiSet,
	id2card map[int]Card,
	trump Trump,
	leadSC SuitClass,
	sub subReq,
) (ok bool, hardFail bool, err error) {
	remainingHand := multisetToCards(handSet, id2card)
	selected, err := FindBlocksInHand(remainingHand, trump, leadSC, sub.bt, sub.tractorLen)
	if err != nil {
		return false, false, fmt.Errorf("藏牌校验出错 %w", err)
	}
	// 手里没有该（子）牌型，允许降级
	if len(selected) == 0 {
		return false, false, nil
	}
	// 过滤出played中能完整覆盖的候选
	valid := make([][]int, 0, len(selected))
	for _, b := range selected {
		ids := blockCardIDs(b)
		if playSet.ContainsAll(ids) {
			// 为了稳定排序：复制并排序 ids
			cp := append([]int(nil), ids...)
			sort.Ints(cp)
			valid = append(valid, cp)
		}
	}
	// 手里有该牌型，但 played 里一个都没跟出来 -> 藏牌（硬失败）
	if len(valid) == 0 {
		return false, true, nil
	}
	// 选一个候选并消费：按 ids 字典序最小（稳定、简单）
	sort.Slice(valid, func(i, j int) bool { return lexLess(valid[i], valid[j]) })
	chosen := valid[0]
	handSet.RemoveAll(chosen)
	playSet.RemoveAll(chosen)
	return true, false, nil
}

func blockCardIDs(b Block) []int {
	ids := make([]int, 0, len(b.Cards))
	for _, c := range b.Cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func lexLess(a, b []int) bool {
	na, nb := len(a), len(b)
	n := na
	if nb < n {
		n = nb
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return na < nb
}

// ============================
// multiset（map[int]int）
// ============================

type multiSet struct {
	m     map[int]int
	total int
}

func newMultiSet() *multiSet {
	return &multiSet{m: make(map[int]int)}
}

func (s *multiSet) Clone() *multiSet {
	cp := newMultiSet()
	cp.total = s.total
	for k, v := range s.m {
		cp.m[k] = v
	}
	return cp
}

func (s *multiSet) Add(id int, n int) {
	if n <= 0 {
		return
	}
	s.m[id] += n
	s.total += n
}

func (s *multiSet) Total() int { return s.total }

func (s *multiSet) ContainsAll(ids []int) bool {
	tmp := make(map[int]int, len(ids))
	for _, id := range ids {
		tmp[id]++
	}
	for id, need := range tmp {
		if s.m[id] < need {
			return false
		}
	}
	return true
}

func (s *multiSet) RemoveAll(ids []int) {
	for _, id := range ids {
		if s.m[id] <= 0 {
			continue
		}
		s.m[id]--
		s.total--
		if s.m[id] == 0 {
			delete(s.m, id)
		}
	}
}

func multisetToCards(ms *multiSet, id2card map[int]Card) []Card {
	out := make([]Card, 0, ms.total)
	for id, cnt := range ms.m {
		c, ok := id2card[id]
		if !ok {
			continue
		}
		for i := 0; i < cnt; i++ {
			out = append(out, c)
		}
	}
	return out
}
