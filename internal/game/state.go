package game

import "upgrade-lan/internal/game/rules"

type Phase string

const (
	PhaseLobby       Phase = "lobby"
	PhaseDealing     Phase = "dealing"
	PhaseCallTrump   Phase = "call_trump"
	PhaseBottom      Phase = "bottom"
	PhaseTrumpFight  Phase = "trump_fight"
	PhasePlayTrick   Phase = "play_trick"
	PhaseRoundSettle Phase = "round_settle"
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
	Blocks  [][]rules.Block `json:"blocks"`    // 牌型二维切片。外层：不同牌型组（拖拉机/对子/单张）；内层：同牌型的若干block，按牌力降序
	CardIDs []int           `json:"actualIds"` // （用于前端）出牌ID
	Cards   []rules.Card    `json:"cards"`     // （用于前端）出牌
}

type PlayedMove struct {
	Move      `json:"followMove"` // 先手出牌/跟牌
	Seat      int                 `json:"seat"`      // 进入 PhasePlayTrick 时初始化为-1，表示未出牌
	SuitClass rules.SuitClass     `json:"suitClass"` // 牌域。若跟牌牌域不一致（垫），则SuitClass = "Mix"，不可参与回合结算
	Info      string              `json:"info"`      // 附加信息
}

type ThrowMove struct {
	IsThrow    bool `json:"isThrow"`    // 先手玩家原意是否甩牌
	ThrowOK    bool `json:"throwOk"`    // 甩牌是否成功（true=保留；false=裁剪）
	IntentMove Move `json:"intentMove"` // 原出牌意图
}

type TrickState struct {
	LeaderSeat int            `json:"leaderSeat"`  // 本回合先手
	TurnSeat   int            `json:"turnSeat"`    // 当前轮到谁
	Plays      [4]*PlayedMove `json:"playedMoves"` // 每座位本回合实际出的牌（未出牌则为空）
	Throw      *ThrowMove     `json:"throwMove"`   // 先手甩牌意图

	BiggerSeat int  `json:"biggerSeat"` // 当前最大者
	Resolved   bool `json:"resolved"`   // 本回合（本墩）是否结束
	WinnerSeat int  `json:"winnerSeat"` // resolved 后有效

	LastPlays [4]*PlayedMove `json:"lastMoves"` // 上一回合的出牌记录
}

type RoundOutcome struct {
	Label           string
	CallerDelta     int // 坐家队升级
	DefenderDelta   int // 打家队升级
	NextStarterSeat int
}

type SuitRecord struct {
	A   int `json:"a"`
	K   int `json:"k"`
	Ten int `json:"ten"`
	Num int `json:"num"`
}

type Record struct {
	Spade      SuitRecord `json:"spade"`
	Heart      SuitRecord `json:"heart"`
	Diamond    SuitRecord `json:"diamond"`
	Club       SuitRecord `json:"club"`
	BigJoker   int        `json:"bigJoker"`
	SmallJoker int        `json:"smallJoker"`
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
	CallPassMask uint8    `json:"-"`          // bit0..bit3 表示seat是否已pass（内部），用于第一小局判定是否无主

	NextStarterSeat int `json:"-"`             // 跨小局保留：下一小局谁先定主/先手（结算时写）
	CallerSeat      int `json:"callerSeat"`    // 本小局谁定主
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
	Trick      TrickState `json:"trick"`
	Points     int        `json:"points"`     // 本墩打家吃分（末墩抠底之前）
	TrickIndex int        `json:"trickIndex"` // 本小局第几墩，从0开始
	HideRecord bool       `json:"hideRecord"`
	Record     Record     `json:"record"` // 记牌功能

	// ---- 末墩抠底 ----
	BottomRevealed bool         `json:"bottomRevealed"`         // 是否已经抠/公开底牌（用于断线重连）
	BottomReveal   []rules.Card `json:"bottomReveal,omitempty"` // 公开给前端展示（可选）
	BottomPoints   int          `json:"bottomPoints"`           // 底牌分（不含倍率）
	BottomMul      int          `json:"bottomMul"`              // 倍率（2/4/1）
	BottomAward    int          `json:"bottomAward"`            // 实际加到 st.Points 的分（含倍率，且只在打家得分时生效）

	// ---- 小局结算展示 ----
	RoundPointsFinal int    `json:"roundPointsFinal"` // = Points（结算时拷贝）
	RoundResultLabel string `json:"roundResultLabel"` // 满分/大胜/过大关/换坐/过小关/不过小关/光头
	CallerDelta      int    `json:"callerDelta"`      // 坐家升级
	DefenderDelta    int    `json:"defenderDelta"`    // 打家升级
}
