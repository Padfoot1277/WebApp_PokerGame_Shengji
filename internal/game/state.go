package game

type Phase string

const (
	PhaseLobby   Phase = "lobby"
	PhaseDealing Phase = "dealing"
)

type SeatState struct {
	UID    string `json:"uid"`
	Ready  bool   `json:"ready"`
	Online bool   `json:"online"`
	// 手牌先不做，后面加：Hand []Card
}

type GameState struct {
	RoomID  string       `json:"roomId"`
	Phase   Phase        `json:"phase"`
	Version int64        `json:"version"`
	Seats   [4]SeatState `json:"seats"`
}
