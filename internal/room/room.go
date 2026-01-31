package room

import (
	"encoding/json"

	"upgrade-lan/internal/game"
	"upgrade-lan/internal/transport"
)

type incoming struct {
	c   transport.Client
	typ string
	raw json.RawMessage
}

type Room struct {
	id string

	join  chan transport.Client
	leave chan transport.Client
	inbox chan incoming

	conns map[transport.Client]struct{}
	state game.GameState
}

func NewRoom(id string) *Room {
	return &Room{
		id:    id,
		join:  make(chan transport.Client, 32),
		leave: make(chan transport.Client, 32),
		inbox: make(chan incoming, 128),
		conns: make(map[transport.Client]struct{}),
		state: game.GameState{
			RoomID: id,
			Phase:  game.PhaseLobby,
		},
	}
}

func (r *Room) Join(c transport.Client)  { r.join <- c }
func (r *Room) Leave(c transport.Client) { r.leave <- c }

func (r *Room) Route(c transport.Client, typ string, raw json.RawMessage) {
	r.inbox <- incoming{c: c, typ: typ, raw: raw}
}

func (r *Room) Run() {
	for {
		select {
		case c := <-r.join:
			r.conns[c] = struct{}{}
			// 若该 uid 已经坐下，标 online
			for i := 0; i < 4; i++ {
				if r.state.Seats[i].UID == c.UID() {
					r.state.Seats[i].Online = true
				}
			}
			r.broadcastSnapshot()

		case c := <-r.leave:
			delete(r.conns, c)
			for i := 0; i < 4; i++ {
				if r.state.Seats[i].UID == c.UID() {
					r.state.Seats[i].Online = false
					r.state.Seats[i].Ready = false
				}
			}
			r.state.Version++
			r.broadcastSnapshot()
			_ = c.Close()

		case msg := <-r.inbox:
			r.handleEvent(msg.c, msg.typ, msg.raw)
		}
	}
}

func (r *Room) handleEvent(c transport.Client, typ string, raw json.RawMessage) {
	evType, payload, err := ParseClientEvent(typ, raw)
	if err != nil {
		_ = c.SendJSON(game.ErrorMsg{Type: "error", Code: "bad_event", Message: err.Error()})
		return
	}
	res, err := game.Reduce(r.state, c.UID(), evType, payload)
	if err != nil {
		_ = c.SendJSON(game.ErrorMsg{Type: "error", Code: "reject", Message: err.Error()})
		return
	}
	if res.Changed {
		r.state = res.State
		r.broadcastSnapshot()
	}
}

func (r *Room) broadcastSnapshot() {
	snap := game.Snapshot{Type: "snapshot", State: r.state}
	for c := range r.conns {
		_ = c.SendJSON(snap)
	}
}
