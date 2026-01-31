package game

type ClientEventType string

const (
	EvSit     ClientEventType = "room.sit"
	EvLeave   ClientEventType = "room.leave_seat"
	EvReady   ClientEventType = "room.ready"
	EvUnready ClientEventType = "room.unready"
	EvStart   ClientEventType = "game.start"
)

type SitPayload struct {
	Seat int `json:"seat"`
}

type ReadyPayload struct {
	Ready bool `json:"ready"`
}
