package game

import (
	"upgrade-lan/internal/game/rules"
)

// --- PayLoad 校验 ---
func validateLen(ids []int, want int) *AppError {
	if len(ids) != want {
		return ErrWrongCardsNum.WithInfof("需打出%d张牌！", want)
	}
	return nil
}

func validateLenIn(ids []int, a, b int) *AppError {
	if len(ids) != a && len(ids) != b {
		return ErrWrongCardsNum.WithInfof("需打出%d或%d张牌！", a, b)
	}
	return nil
}

func validateUnique(ids []int) *AppError {
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return ErrDuplicateIDs.WithInfof("重复打出了牌，ID%d！", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateNonEmpty(ids []int) *AppError {
	if len(ids) == 0 {
		return ErrEmptyCards.WithInfof("出牌数为0！")
	}
	return nil
}

// --- Reduce 工具函数 ---

func allReady(st *GameState) bool {
	for i := 0; i < 4; i++ {
		if st.Seats[i].UID == "" || !st.Seats[i].Ready {
			return false
		}
	}
	return true
}

// TeamOfSeat seat0&2 -> team0; seat1&3 -> team1
func TeamOfSeat(seat int) int {
	if seat%2 == 0 {
		return 0
	}
	return 1
}

func seatIndexByUID(st *GameState, uid string) (int, *AppError) {
	for i := 0; i < 4; i++ {
		if st.Seats[i].UID == uid {
			return i, nil
		}
	}
	return -1, ErrStateNotSeated.WithInfof("请先选择位置并准备")
}

type CardIndex map[int]rules.Card

func NewCardIndex(cards []rules.Card) CardIndex {
	idx := make(CardIndex, len(cards))
	for _, c := range cards {
		idx[c.ID] = c
	}
	return idx
}

func (idx CardIndex) Get(id int) (rules.Card, bool) {
	c, ok := idx[id]
	return c, ok
}

func getIDs(cards []rules.Card) []int {
	ids := make([]int, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func sortAllHands(st *GameState) {
	for i := 0; i < 4; i++ {
		rules.SortHand(st.Seats[i].Hand, st.Trump.Trump)
	}
}

// 在定主/改主/攻主后，重新修改每张牌的SuitClass
func refreshHandSuitClassForUI(st *GameState) {
	for i := 0; i < 4; i++ {
		for j := range st.Seats[i].Hand {
			st.Seats[i].Hand[j].SuitClass = rules.ComputeSuitClass(st.Seats[i].Hand[j], st.Trump.Trump)
		}
	}
}

// pickCardsFromHand 传入牌号，返回牌组
func pickCardsFromHand(hand []rules.Card, selectedIDs []int) ([]rules.Card, *AppError) {
	handIdx := NewCardIndex(hand)
	seen := map[int]bool{}
	selectedCards := make([]rules.Card, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		if seen[id] {
			return nil, ErrDuplicateIDs.WithInfof("重复打出了牌，ID%d", id)
		}
		seen[id] = true
		c, ok := handIdx.Get(id)
		if !ok {
			return nil, ErrRuleIllegalPlay.WithInfof("所出牌非手牌，ID%d", id)
		}
		selectedCards = append(selectedCards, c)
	}
	return selectedCards, nil
}

// deleteCardFromHand 从手牌中删除所选牌（selected）
func deleteCardsFromHand(hand []rules.Card, ids []int) []rules.Card {
	if len(ids) == 0 {
		return hand
	}
	rm := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		rm[id] = struct{}{}
	}
	keep := make([]rules.Card, 0, len(hand)-len(ids))
	for _, c := range hand {
		if _, ok := rm[c.ID]; ok {
			continue
		}
		keep = append(keep, c)
	}
	return keep
}

func cloneMove(m Move) Move {
	cp := m
	if m.Blocks != nil {
		cp.Blocks = make([][]rules.Block, len(m.Blocks))
		for i := range m.Blocks {
			cp.Blocks[i] = append([]rules.Block(nil), m.Blocks[i]...)
		}
	}
	cp.CardIDs = append([]int(nil), m.CardIDs...)
	cp.Cards = append([]rules.Card(nil), m.Cards...)
	return cp
}
