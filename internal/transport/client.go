package transport

// Client 是 room 层唯一关心的“连接抽象”。
// ws.Conn 去实现它，但 room 不需要 import ws。
type Client interface {
	UID() string
	RoomID() string
	SendJSON(v any) error
	Close() error
}
