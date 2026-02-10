package game

import "upgrade-lan/internal/game/rules"

type Snapshot struct {
	Type  string    `json:"type"` // "snapshot"
	State ViewState `json:"state"`
}

type ViewState struct {
	RoomID  string      `json:"roomId"`
	Phase   Phase       `json:"phase"`
	Version int64       `json:"version"`
	Seats   [4]SeatView `json:"seats"`
	Teams   [2]TeamView `json:"teams"`

	RoundIndex       int      `json:"roundIndex"`
	CallMode         CallMode `json:"callMode"`
	CallPassedSeats  [4]bool  `json:"callPassedSeats"`
	StarterSeat      int      `json:"starterSeat"`
	CallTurnSeat     int      `json:"callTurnSeat"`
	CallPassCount    int      `json:"callPassCount"`
	FightPassedSeats [4]bool  `json:"fightPassedSeats"`
	FightPassCount   int      `json:"fightPassCount"`
	BottomOwnerSeat  int      `json:"bottomOwnerSeat"`

	Trump  TrumpState `json:"trump"`
	Trick  TrickState `json:"trick"`  // 全部可见
	Points int        `json:"points"` // 每小局打家的得分

	MySeat   int          `json:"mySeat"`
	MyBottom []rules.Card `json:"myBottom"` // 仅在 PhaseBottom 本人可见
	MyHand   []rules.Card `json:"myHand"`   // 仅本人可见
}

type SeatView struct {
	UID       string `json:"uid"`
	Ready     bool   `json:"ready"`
	Online    bool   `json:"online"`
	Team      int    `json:"team"`
	HandCount int    `json:"handCount"`
}

type TeamView struct {
	LevelRank rules.Rank `json:"levelRank"`
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type NoticeMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// MakeView 后端永远保存完整 state，但下发永远走 view
func MakeView(st GameState, uid string) ViewState {
	var seats [4]SeatView
	var teams [2]TeamView

	for t := 0; t < 2; t++ {
		teams[t] = TeamView{LevelRank: st.Teams[t].LevelRank}
	}

	myHand := []rules.Card(nil)
	myBottom := []rules.Card(nil)
	mySeat := -1

	for i := 0; i < 4; i++ {
		seats[i] = SeatView{
			UID:       st.Seats[i].UID,
			Ready:     st.Seats[i].Ready,
			Online:    st.Seats[i].Online,
			Team:      st.Seats[i].Team,
			HandCount: st.Seats[i].HandCount,
		}
		if st.Seats[i].UID == uid {
			mySeat = i
			myHand = append([]rules.Card(nil), st.Seats[i].Hand...)
		}
	}

	passed := maskToBool4(st.CallPassMask)
	fightPassed := maskToBool4(st.FightPassMask)

	// 私有：只有坐家在扣底阶段能看到底牌牌面
	if st.Phase == PhaseBottom && st.BottomOwnerSeat >= 0 {
		ownerUID := st.Seats[st.BottomOwnerSeat].UID
		if ownerUID == uid {
			myBottom = append([]rules.Card(nil), st.Bottom...)
		}
	}

	return ViewState{
		RoomID:  st.RoomID,
		Phase:   st.Phase,
		Version: st.Version,
		Seats:   seats,
		Teams:   teams,

		RoundIndex:      st.RoundIndex,
		CallMode:        st.CallMode,
		CallPassedSeats: passed,

		StarterSeat:   st.CallerSeat,
		CallTurnSeat:  st.CallTurnSeat,
		CallPassCount: st.CallPassCount,

		FightPassedSeats: fightPassed,
		FightPassCount:   st.FightPassCount,
		Trump:            st.Trump,

		BottomOwnerSeat: st.BottomOwnerSeat,

		MySeat:   mySeat,
		MyBottom: myBottom,
		MyHand:   myHand,

		Trick:  st.Trick,
		Points: st.Points,
	}
}

func maskToBool4(m uint8) (out [4]bool) {
	for i := 0; i < 4; i++ {
		out[i] = (m & (1 << uint(i))) != 0
	}
	return
}
