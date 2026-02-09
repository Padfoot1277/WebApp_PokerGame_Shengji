package room

import (
	"encoding/json"
	"upgrade-lan/internal/game"
)

func ParseClientEvent(typ string, raw json.RawMessage) (game.ClientEventType, any, *game.AppError) {
	switch typ {
	case string(game.EvSit):
		var p game.SitPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, game.ErrBadJSON.WithCause("选座请求解析错误")
		}
		if err := p.Validate(); err != nil {
			return "", nil, game.ErrInvalidPayload.WithCause(err.Error())
		}
		return game.EvSit, p, nil
	case string(game.EvLeave):
		return game.EvLeave, struct{}{}, nil
	case string(game.EvReady):
		return game.EvReady, struct{}{}, nil
	case string(game.EvUnready):
		return game.EvUnready, struct{}{}, nil
	case string(game.EvStart):
		return game.EvStart, struct{}{}, nil

	case string(game.EvCallPass):
		return game.EvCallPass, struct{}{}, nil
	case string(game.EvCallTrump):
		var p game.CallTrumpPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, game.ErrBadJSON.WithCause("定主请求解析错误")
		}
		if err := p.Validate(); err != nil {
			return "", nil, game.ErrInvalidPayload.WithCause(err.Error())
		}
		return game.EvCallTrump, p, nil

	case string(game.EvPutBottom):
		var p game.PutBottomPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, game.ErrBadJSON.WithCause("扣底请求解析错误")
		}
		if err := p.Validate(); err != nil {
			return "", nil, game.ErrInvalidPayload.WithCause(err.Error())
		}
		return game.EvPutBottom, p, nil

	case string(game.EvChangeTrump):
		var p game.ChangeTrumpPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, game.ErrBadJSON.WithCause("改主请求解析错误")
		}
		if err := p.Validate(); err != nil {
			return "", nil, game.ErrInvalidPayload.WithCause(err.Error())
		}
		return game.EvChangeTrump, p, nil

	case string(game.EvAttackTrump):
		var p game.AttackTrumpPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, game.ErrBadJSON.WithCause("攻主请求解析错误")
		}
		if err := p.Validate(); err != nil {
			return "", nil, game.ErrInvalidPayload.WithCause(err.Error())
		}
		return game.EvAttackTrump, p, nil

	case string(game.EvPlayCards):
		var p game.PlayCardsPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, game.ErrBadJSON.WithCause("出牌请求解析错误")
		}
		if err := p.Validate(); err != nil {
			return "", nil, game.ErrInvalidPayload.WithCause(err.Error())
		}
		return game.EvPlayCards, p, nil

	default:
		return "", nil, game.ErrUnknownEvent
	}
}
