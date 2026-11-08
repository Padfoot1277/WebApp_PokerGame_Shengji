package rules

import (
	"fmt"
)

// ValidateHideCheck enforces equal card counts and suit priority, then compares
// the best attainable tractor/pair profile of the hand with the submitted cards.
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

	reqs, err := flattenLeadReqs(leadMove)
	if err != nil {
		return err
	}
	// Pair resources are allocated globally. Singles fill the remaining slots, so they
	// cannot accidentally consume a pair needed by a later structure.
	want, err := bestFollowProfile(handLead, reqs, trump, leadSC)
	if err != nil {
		return err
	}
	got, err := bestFollowProfile(played, reqs, trump, leadSC)
	if err != nil {
		return err
	}
	if compareProfile(got, want) != 0 {
		return fmt.Errorf("不可藏牌，需尽可能跟出同结构的牌")
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

// A profile records the number of tractors at each length, then pairs, for each
// lead block in priority order. Memoization bounds the search by pair subsets.
func bestFollowProfile(cards []Card, reqs []reqBlock, trump Trump, sc SuitClass) ([]int, error) {
	pairs, err := buildPairs(cards, trump, sc)
	if err != nil {
		return nil, err
	}
	type key struct{ req, mask int }
	memo := map[key][]int{}
	var solve func(int, int) []int
	solve = func(index, mask int) []int {
		if index == len(reqs) {
			return []int{}
		}
		k := key{index, mask}
		if cached, ok := memo[k]; ok {
			return cached
		}
		req := reqs[index]
		maxLen := 1
		if req.bt == BlockTractor {
			maxLen = req.tractorLen
		}
		if req.bt == BlockSingle {
			result := solve(index+1, mask)
			memo[k] = result
			return result
		}
		profile := make([]int, maxLen)
		var best []int
		visited := map[string]bool{}
		var allocate func(int, int, int)
		allocate = func(length, budget, available int) {
			state := fmt.Sprint(length, ":", budget, ":", available, ":", profile)
			if visited[state] {
				return
			}
			visited[state] = true
			if length == 0 {
				candidate := append(append([]int{}, profile...), solve(index+1, available)...)
				if best == nil || compareProfile(candidate, best) > 0 {
					best = candidate
				}
				return
			}
			// Skip this size after enumerating zero or more disjoint groups of it.
			allocate(length-1, budget, available)
			if budget < length {
				return
			}
			seen := map[int]bool{}
			for i := range pairs {
				if available&(1<<i) == 0 {
					continue
				}
				var extend func(int, int, int)
				extend = func(last, remaining, used int) {
					if remaining == 0 {
						if seen[used] {
							return
						}
						seen[used] = true
						profile[maxLen-length]++
						allocate(length, budget-length, available&^used)
						profile[maxLen-length]--
						return
					}
					for j := range pairs {
						if available&(1<<j) != 0 && used&(1<<j) == 0 && pairs[j].RankValue == pairs[last].RankValue-1 {
							extend(j, remaining-1, used|1<<j)
						}
					}
				}
				extend(i, length-1, 1<<i)
			}
		}
		allocate(maxLen, req.needCards/2, mask)
		memo[k] = best
		return best
	}
	return solve(0, (1<<len(pairs))-1), nil
}
func compareProfile(a, b []int) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
