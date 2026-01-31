package game

type ClientEventType string

const (
	EvSit     ClientEventType = "room.sit"
	EvLeave   ClientEventType = "room.leave_seat"
	EvReady   ClientEventType = "room.ready"
	EvUnready ClientEventType = "room.unready"

	EvStart ClientEventType = "game.start"

	EvCallTrump   ClientEventType = "game.call_trump"
	EvCallPass    ClientEventType = "game.call_pass"
	EvPutBottom   ClientEventType = "game.put_bottom"
	EvChangeTrump ClientEventType = "game.change_trump"
	EvAttackTrump ClientEventType = "game.attack_trump"
)

type SitPayload struct {
	Seat int `json:"seat"`
}

// CallTrumpPayload 定主：公开用哪些牌定主
// levelIds: 1张表示普通定主；2张表示“一对级牌” -> 触发锁主（同色王 + 一对级牌）
type CallTrumpPayload struct {
	JokerID  int   `json:"jokerId"`
	LevelIDs []int `json:"levelIds"` // len=1 or 2
}

type PutBottomPayload struct {
	DiscardIDs []int `json:"discardIds"` // 必须正好8张，从33张手牌里选
}

type ChangeTrumpPayload struct {
	JokerID  int   `json:"jokerId"`
	LevelIDs []int `json:"levelIds"` // 必须2张：同花色、同rank
}

type AttackTrumpPayload struct {
	JokerIDs []int `json:"jokerIds"` // 必须2张：同 kind（big/big 或 small/small）
}
