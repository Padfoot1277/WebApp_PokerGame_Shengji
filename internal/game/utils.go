package game

import (
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

func findCardByID(hand []rules.Card, id int) (rules.Card, bool) {
	for _, c := range hand {
		if c.ID == id {
			return c, true
		}
	}
	return rules.Card{}, false
}

func sortAllHands(st GameState) {
	for i := 0; i < 4; i++ {
		rules.SortHand(st.Seats[i].Hand, rules.SortCtx{
			LevelRank:    st.Trump.LevelRank,
			HasTrumpSuit: st.Trump.HasTrumpSuit,
			TrumpSuit:    st.Trump.Suit,
		})
	}
}
