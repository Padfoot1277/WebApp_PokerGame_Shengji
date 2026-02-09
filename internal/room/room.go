package room

import (
	"encoding/json"
	"fmt"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
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
	st := game.GameState{
		RoomID: id,
		Phase:  game.PhaseLobby,
	}
	// 初始化座位所属队伍
	for i := 0; i < 4; i++ {
		st.Seats[i].Team = game.TeamOfSeat(i)
	}
	// 初始化双方级牌 = 2
	st.Teams[0].LevelRank = rules.R2
	st.Teams[1].LevelRank = rules.R2

	st.RoundIndex = 0
	st.NextStarterSeat = 0 // 后续小局用（结算写回）
	st.StarterSeat = -1    //
	st.CallTurnSeat = -1   // 首局抢定主不需要turn
	st.CallPassCount = 0
	st.CallPassMask = 0
	st.CallMode = game.CallModeRace // 首局抢定主
	st.BottomOwnerSeat = -1
	st.Trump.CallerSeat = -1

	return &Room{
		id:    id,
		join:  make(chan transport.Client, 32),
		leave: make(chan transport.Client, 32),
		inbox: make(chan incoming, 128),
		conns: make(map[transport.Client]struct{}),
		state: st,
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
			c.SendJSON(map[string]any{
				"type": "hello",
				"uid":  c.UID(),
			})
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
		_ = c.SendJSON(game.ErrorMsg{Type: "error", Message: err.Error()})
		return
	}
	res, err := game.Reduce(r.state, c.UID(), evType, payload)
	if err != nil {
		fmt.Println(err, res.Notice)
		_ = c.SendJSON(game.ErrorMsg{Type: "error", Message: err.Error()})
		return
	}
	if res.Changed {
		fmt.Println(res.Notice)
		r.state = res.State
		r.broadcastSnapshot()
	}
}

func (r *Room) broadcastSnapshot() {
	for c := range r.conns {
		view := game.MakeView(r.state, c.UID())
		snap := game.Snapshot{Type: "snapshot", State: view}
		_ = c.SendJSON(snap)
	}
}
