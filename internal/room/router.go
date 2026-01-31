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
	default:
		return "", nil, fmt.Errorf("unknown_event_type: %s", typ)
	}
}
