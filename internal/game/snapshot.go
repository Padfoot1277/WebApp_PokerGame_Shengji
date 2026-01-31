package game

import "upgrade-lan/internal/game/rules"

type Snapshot struct {
	Type  string    `json:"type"` // "snapshot"
	State ViewState `json:"state"`
}

type ViewState struct {
	RoomID      string      `json:"roomId"`
	Phase       Phase       `json:"phase"`
	Version     int64       `json:"version"`
	Seats       [4]SeatView `json:"seats"`
	Teams       [2]TeamView `json:"teams"`
	BottomCount int         `json:"bottomCount"`

	MyHand []rules.Card `json:"myHand"` // 只给本人
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
	Type    string `json:"type"` // "error"
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MakeView 后端永远保存完整 state，但下发永远走 view
func MakeView(st GameState, uid string) ViewState {
	var seats [4]SeatView
	var teams [2]TeamView
	myHand := []rules.Card(nil)

	for t := 0; t < 2; t++ {
		teams[t] = TeamView{LevelRank: st.Teams[t].LevelRank}
	}

	for i := 0; i < 4; i++ {
		seats[i] = SeatView{
			UID:       st.Seats[i].UID,
			Ready:     st.Seats[i].Ready,
			Online:    st.Seats[i].Online,
			Team:      st.Seats[i].Team,
			HandCount: st.Seats[i].HandCount,
		}
		if st.Seats[i].UID == uid {
			myHand = append([]rules.Card(nil), st.Seats[i].Hand...)
		}
	}

	return ViewState{
		RoomID:      st.RoomID,
		Phase:       st.Phase,
		Version:     st.Version,
		Seats:       seats,
		Teams:       teams,
		BottomCount: st.BottomCount,
		MyHand:      myHand,
	}
}
