package room

import (
	"encoding/json"
	"sync"

	"upgrade-lan/internal/transport"
)

type Manager struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

func (m *Manager) getOrCreate(roomID string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.rooms[roomID]; ok {
		return r
	}
	r := NewRoom(roomID)
	m.rooms[roomID] = r
	go r.Run()
	return r
}

// —— 实现 ws.Router 接口（但这里不 import ws，因为接口在 ws 包里定义）
// 为了不 import ws，我们直接让 ws.Router 依赖 transport.Client，
// main.go 里传 rm 给 ws.ServeWS 即可（编译器会检查方法集匹配）。

func (m *Manager) OnConnect(c transport.Client) {
	r := m.getOrCreate(c.RoomID())
	r.Join(c)
}

func (m *Manager) OnDisconnect(c transport.Client) {
	m.mu.Lock()
	r := m.rooms[c.RoomID()]
	m.mu.Unlock()
	if r != nil {
		r.Leave(c)
	}
}

func (m *Manager) OnMessage(c transport.Client, typ string, payload json.RawMessage) {
	r := m.getOrCreate(c.RoomID())
	r.Route(c, typ, payload)
}
