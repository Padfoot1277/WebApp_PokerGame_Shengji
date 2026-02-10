package game

type ClientEventType string

const (
	EvSit     ClientEventType = "room.sit"
	EvLeave   ClientEventType = "room.leave_seat"
	EvReady   ClientEventType = "room.ready"
	EvUnready ClientEventType = "room.unready"

	EvStart          ClientEventType = "game.start"
	EvStartNextRound ClientEventType = "game.start_next_round"

	EvCallTrump   ClientEventType = "game.call_trump"
	EvCallPass    ClientEventType = "game.call_pass"
	EvPutBottom   ClientEventType = "game.put_bottom"
	EvChangeTrump ClientEventType = "game.change_trump"
	EvAttackTrump ClientEventType = "game.attack_trump"

	EvPlayCards ClientEventType = "game.play_cards"
)

// ---- 请求Payload ----

type SitPayload struct {
	Seat int `json:"seat"`
}

// CallTrumpPayload 定主：公开用哪些牌定主
// levelIds: 1张表示普通定主；2张表示“一对级牌” -> 触发锁主（同色王 + 一对级牌）
type CallTrumpPayload struct {
	JokerID  int   `json:"jokerId"`
	LevelIDs []int `json:"levelIds"` // len=1 or 2
}

type ChangeTrumpPayload struct {
	JokerID  int   `json:"jokerId"`
	LevelIDs []int `json:"levelIds"` // 必须2张：同花色、同rank
}

type AttackTrumpPayload struct {
	JokerIDs []int `json:"jokerIds"` // 必须2张：同 kind（big/big 或 small/small）
}

type PutBottomPayload struct {
	DiscardIDs []int `json:"discardIds"` // 必须正好8张，从33张手牌里选
}

type PlayCardsPayload struct {
	CardIDs []int `json:"cardIds"`
}

// ---- PayLoad 校验 ----

func (p SitPayload) Validate() *AppError {
	if p.Seat < 0 || p.Seat >= 4 {
		return ErrSeatRange
	}
	return nil
}

func (p CallTrumpPayload) Validate() *AppError {
	if err := validateLenIn(p.LevelIDs, 1, 2); err != nil {
		return err
	}
	if err := validateUnique(p.LevelIDs); err != nil {
		return err
	}
	if p.JokerID < 0 {
		return ErrWrongCardsNum.WithInfo("缺少定主王牌")
	}
	return nil
}

func (p ChangeTrumpPayload) Validate() *AppError {
	if err := validateLen(p.LevelIDs, 2); err != nil {
		return err
	}
	if err := validateUnique(p.LevelIDs); err != nil {
		return err
	}
	if p.JokerID < 0 {
		return ErrWrongCardsNum.WithInfo("缺少定主王牌")
	}
	return nil
}

func (p AttackTrumpPayload) Validate() *AppError {
	if err := validateLen(p.JokerIDs, 2); err != nil {
		return err
	}
	if err := validateUnique(p.JokerIDs); err != nil {
		return err
	}
	return nil
}

func (p PutBottomPayload) Validate() *AppError {
	if err := validateLen(p.DiscardIDs, 8); err != nil {
		return err
	}
	if err := validateUnique(p.DiscardIDs); err != nil {
		return err
	}
	return nil
}

func (p PlayCardsPayload) Validate() *AppError {
	if err := validateNonEmpty(p.CardIDs); err != nil {
		return err
	}
	if err := validateUnique(p.CardIDs); err != nil {
		return err
	}
	return nil
}
