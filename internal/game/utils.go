package game

import (
	"fmt"
	"upgrade-lan/internal/game/rules"
)

func allReady(st GameState) bool {
	for i := 0; i < 4; i++ {
		if st.Seats[i].UID == "" || !st.Seats[i].Ready {
			return false
		}
	}
	return true
}

func seatIndexByUID(st GameState, uid string) (int, bool) {
	for i := 0; i < 4; i++ {
		if st.Seats[i].UID == uid {
			return i, true
		}
	}
	return -1, false
}

func findCardByID(cards []rules.Card, id int) (rules.Card, bool) {
	for _, c := range cards {
		if c.ID == id {
			return c, true
		}
	}
	return rules.Card{}, false
}

func getIDs(cards []rules.Card) (ids []int) {
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func sortAllHands(st GameState) GameState {
	for i := 0; i < 4; i++ {
		rules.SortHand(st.Seats[i].Hand, rules.SortCtx{
			LevelRank:    st.Trump.LevelRank,
			HasTrumpSuit: st.Trump.HasTrumpSuit,
			TrumpSuit:    st.Trump.Suit,
		})
		// 每次重牌后更新每张牌的牌域
		for j := range st.Seats[i].Hand {
			st.Seats[i].Hand[j].SuitClass = rules.ComputeSuitClass(st.Seats[i].Hand[j], st.Trump.Trump)
		}
	}
	return st
}

// checkAllInHand 传入牌号，检查所出牌是否都在手牌中，返回最终牌组
func checkAllInHand(selectedIDs []int, hand []rules.Card) ([]rules.Card, error) {
	inHand := map[int]rules.Card{}
	for _, c := range hand {
		inHand[c.ID] = c
	}
	seen := map[int]bool{}
	selectedCards := make([]rules.Card, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		if seen[id] {
			return nil, fmt.Errorf("重复打出相同牌")
		}
		seen[id] = true
		c, ok := inHand[id]
		if !ok {
			return nil, fmt.Errorf("所出牌非手牌")
		}
		selectedCards = append(selectedCards, c)
	}
	return selectedCards, nil
}

// deleteCardFromHand 从手牌中删除所选牌（以保证合法性）
func deleteCardFromHand(hand, selected []rules.Card) []rules.Card {
	keep := make([]rules.Card, 0, len(hand)-len(selected))
	for _, c := range hand {
		if _, ok := findCardByID(selected, c.ID); ok {
			continue
		}
		keep = append(keep, c)
	}
	return keep
}

func cloneMove(m Move) Move {
	cp := m
	cp.Blocks = append([][]rules.Block(nil), m.Blocks...)
	cp.CardIDs = append([]int(nil), m.CardIDs...)
	cp.Cards = append([]rules.Card(nil), m.Cards...)
	return cp
}
