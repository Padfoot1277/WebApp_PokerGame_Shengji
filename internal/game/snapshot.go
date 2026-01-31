package game

type Snapshot struct {
	Type  string    `json:"type"` // "snapshot"
	State GameState `json:"state"`
}

type ErrorMsg struct {
	Type    string `json:"type"` // "error"
	Code    string `json:"code"`
	Message string `json:"message"`
}
