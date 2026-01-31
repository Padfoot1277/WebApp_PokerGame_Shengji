package game

import "upgrade-lan/internal/game/rules"

type Phase string

const (
	PhaseLobby     Phase = "lobby"
	PhaseDealing   Phase = "dealing"
	PhaseCallTrump Phase = "call_trump"
)

type SeatState struct {
	UID    string `json:"uid"`
	Ready  bool   `json:"ready"`
	Online bool   `json:"online"`

	Team int `json:"team"` // 0 or 1（固定对家映射）

	HandCount int          `json:"handCount"` // 对外公开：只显示数量
	Hand      []rules.Card `json:"-"`         // 后端私有：不直接出现在snapshot里
}

type TeamState struct {
	LevelRank rules.Rank `json:"levelRank"` // 本队级牌（初始 2）
}

type GameState struct {
	RoomID  string `json:"roomId"`
	Phase   Phase  `json:"phase"`
	Version int64  `json:"version"`

	Seats [4]SeatState `json:"seats"`
	Teams [2]TeamState `json:"teams"`

	BottomCount int          `json:"bottomCount"` // 对外公开：底牌数量（8）
	Bottom      []rules.Card `json:"-"`           // 后端私有：底牌牌面
}

// TeamOfSeat seat0&2 -> team0; seat1&3 -> team1
func TeamOfSeat(seat int) int {
	if seat%2 == 0 {
		return 0
	}
	return 1
}
