package game

import "upgrade-lan/internal/game/rules"

type Phase string

const (
	PhaseLobby      Phase = "lobby"
	PhaseDealing    Phase = "dealing"
	PhaseCallTrump  Phase = "call_trump"
	PhaseBottom     Phase = "bottom"
	PhaseTrumpFight Phase = "trump_fight"
	PhasePlayTrick  Phase = "play_trick"
)

type SeatState struct {
	UID    string `json:"uid"`
	Ready  bool   `json:"ready"`
	Online bool   `json:"online"`
	Team   int    `json:"team"` // 0 or 1（固定对家映射）

	HandCount int          `json:"handCount"` // 对外公开：只显示数量
	Hand      []rules.Card `json:"-"`         // 后端私有：不直接出现在snapshot里
}

type TeamState struct {
	LevelRank rules.Rank `json:"levelRank"` // 本队级牌（初始 2）
}

type TrumpState struct {
	HasTrumpSuit bool       `json:"hasTrumpSuit"`
	Suit         rules.Suit `json:"suit,omitempty"` // HasTrumpSuit=true 时有效

	LevelRank  rules.Rank `json:"levelRank"`  // 本小局最终级牌（=坐家级数）；硬主时也可填入“本小局先手所在组级牌”作为展示用
	Locked     bool       `json:"locked"`     // 同色王 + 一对级牌 -> 锁主
	CallerSeat int        `json:"callerSeat"` // -1 表示无人定主（硬主）
}

type CallMode string

const (
	CallModeRace    CallMode = "race"    // 抢定主（仅用于第一小局）
	CallModeOrdered CallMode = "ordered" // 按序定主
)

type GameState struct {
	RoomID  string `json:"roomId"`
	Phase   Phase  `json:"phase"`
	Version int64  `json:"version"`

	Seats [4]SeatState `json:"seats"`
	Teams [2]TeamState `json:"teams"`

	// ---- 小局起始/定主流转信息 ----
	RoundIndex   int      `json:"roundIndex"` // 第几小局，从0开始
	CallMode     CallMode `json:"callMode"`   // race / ordered
	CallPassMask uint8    `json:"-"`          // bit0..bit3 表示 seat 是否已pass（内部），用于第一小局判定是否无主

	NextStarterSeat int `json:"-"`             // 跨小局保留：下一小局谁先定主/先手（结算时写）
	StarterSeat     int `json:"starterSeat"`   // 本小局谁先定主（=NextStarterSeat）
	CallTurnSeat    int `json:"callTurnSeat"`  // 当前轮到谁定主
	CallPassCount   int `json:"callPassCount"` // 已pass次数（最多4）

	FightPassMask  uint8 `json:"-"` // 改主攻主
	FightPassCount int   `json:"fightPassCount"`

	Trump TrumpState `json:"trump"`

	// ---- 底牌 ----
	BottomCount     int          `json:"bottomCount"`
	Bottom          []rules.Card `json:"-"`
	BottomOwnerSeat int          `json:"bottomOwnerSeat"`
}

// TeamOfSeat seat0&2 -> team0; seat1&3 -> team1
func TeamOfSeat(seat int) int {
	if seat%2 == 0 {
		return 0
	}
	return 1
}
