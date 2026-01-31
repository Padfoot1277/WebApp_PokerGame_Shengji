package room

import (
	"encoding/json"
	"fmt"

	"upgrade-lan/internal/game"
)

func ParseClientEvent(typ string, raw json.RawMessage) (game.ClientEventType, any, error) {
	switch typ {
	case string(game.EvSit):
		var p game.SitPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, fmt.Errorf("bad_payload sit")
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
			return "", nil, fmt.Errorf("定主请求错误")
		}
		return game.EvCallTrump, p, nil

	case string(game.EvPutBottom):
		var p game.PutBottomPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, fmt.Errorf("扣底请求错误")
		}
		return game.EvPutBottom, p, nil

	case string(game.EvChangeTrump):
		var p game.ChangeTrumpPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, fmt.Errorf("改主请求错误")
		}
		return game.EvChangeTrump, p, nil

	case string(game.EvAttackTrump):
		var p game.AttackTrumpPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", nil, fmt.Errorf("攻主请求错误")
		}
		return game.EvAttackTrump, p, nil
		
	default:
		return "", nil, fmt.Errorf("unknown_event_type: %s", typ)
	}
}
