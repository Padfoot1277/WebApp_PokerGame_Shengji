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
	rules.Trump
	Locked     bool `json:"locked"`     // 同色王 + 一对级牌 -> 锁主
	CallerSeat int  `json:"callerSeat"` // -1 表示无人定主（硬主）
}

type CallMode string

const (
	CallModeRace    CallMode = "race"    // 抢定主（仅用于第一小局）
	CallModeOrdered CallMode = "ordered" // 按序定主
)

type Move struct {
	Blocks  [][]rules.Block `json:"blocks"`    // 牌型（每种牌型合为一组，组内按牌级降序排序，用于判断牌力大小）
	CardIDs []int           `json:"actualIds"` // 出牌ID
	Cards   []rules.Card    `json:"cards"`     // 出牌
}

type LeadMove struct {
	Seat       int             `json:"seat"` // 进入 PhasePlayTrick 时初始化为-1，表示未出牌，便于前端判断
	SuitClass  rules.SuitClass `json:"suitClass"`
	IsThrow    bool            `json:"isThrow"`    // 玩家原意是否甩牌
	ThrowOK    bool            `json:"throwOk"`    // 甩牌是否成功（true=保留；false=裁剪）
	IntentMove Move            `json:"intentMove"` // 原出牌意图
	ActualMove Move            `json:"actualMove"` // 最终出牌
	Info       string          `json:"info"`       // throw_failed_by_xxx 等调试信息
}

type FollowMove struct {
	Seat      int             `json:"seat"`       // 进入 PhasePlayTrick 时初始化为-1，表示未出牌，便于前端判断
	SuitClass rules.SuitClass `json:"suitClass"`  // 若跟牌牌域不一致，则SuitClass = "Mix"，不可参与回合结算
	Move      Move            `json:"followMove"` // 跟牌
	Info      string          `json:"info"`       // 调试信息
}

type TrickState struct {
	LeaderSeat int           `json:"leaderSeat"` // 本回合先手
	TurnSeat   int           `json:"turnSeat"`   // 当前轮到谁
	Lead       LeadMove      `json:"lead"`
	Follows    [3]FollowMove `json:"follows"` // 每座位本回合实际出的牌（未出牌则 CardIDs 为空）
}

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

	// ---- 回合 ----
	Trick TrickState `json:"trick"`
}

// TeamOfSeat seat0&2 -> team0; seat1&3 -> team1
func TeamOfSeat(seat int) int {
	if seat%2 == 0 {
		return 0
	}
	return 1
}
