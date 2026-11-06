package room

import (
	"encoding/json"
	"log/slog"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
	"upgrade-lan/internal/transport"
)

type incoming struct {
	c     transport.Client
	typ   string
	raw   json.RawMessage
	reqID string
}

type Room struct {
	id string

	join  chan transport.Client
	leave chan transport.Client
	inbox chan incoming

	conns      map[transport.Client]struct{}
	state      game.GameState
	receipts   map[string][]receipt
	lastActive time.Time
	done       chan struct{}
	retire     func()
	notice     string
	loadErr    error
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
	st.CallerSeat = -1     //
	st.CallTurnSeat = -1   // 首局抢定主不需要turn
	st.CallPassCount = 0
	st.CallPassMask = 0
	st.CallMode = game.CallModeRace // 首局抢定主
	st.BottomOwnerSeat = -1
	st.Trump.CallerSeat = -1

	r := &Room{
		id:       id,
		receipts: make(map[string][]receipt), lastActive: time.Now(), done: make(chan struct{}),
		join:  make(chan transport.Client),
		leave: make(chan transport.Client, 32),
		inbox: make(chan incoming, 128),
		conns: make(map[transport.Client]struct{}),
		state: st,
	}
	r.loadErr = r.load()
	return r
}

func (r *Room) Join(c transport.Client) {
	select {
	case r.join <- c:
	case <-r.done:
		_ = c.Close()
	}
}
func (r *Room) Leave(c transport.Client) {
	select {
	case r.leave <- c:
	case <-r.done:
	}
}

func (r *Room) Route(c transport.Client, typ string, raw json.RawMessage, reqID string) {
	select {
	case r.inbox <- incoming{c: c, typ: typ, raw: raw, reqID: reqID}:
	case <-r.done:
	}
}

func (r *Room) Run() {
	defer close(r.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(r.conns) == 0 && time.Since(r.lastActive) > 30*time.Minute {
				if r.retire != nil {
					r.retire()
				}
				return
			}
		case c := <-r.join:
			r.lastActive = time.Now()
			if r.loadErr != nil {
				slog.Error("room recovery failed", "room", r.id, "error", r.loadErr)
				_ = c.Close()
				continue
			}

			for old := range r.conns {
				if old.UID() == c.UID() && old != c {
					delete(r.conns, old)
					_ = old.Close()
				}
			}
			if len(r.conns) >= 32 {
				_ = c.Close()
				continue
			}
			r.conns[c] = struct{}{}
			// 若该 uid 已经坐下，标 online
			for i := 0; i < 4; i++ {
				if r.state.Seats[i].UID == c.UID() {
					r.state.Seats[i].Online = true
					r.state.Seats[i].Name = clientName(c)
				}
			}
			c.SendJSON(map[string]any{
				"type":  "hello",
				"uid":   c.UID(),
				"token": clientToken(c),
				"name":  clientName(c),
			})
			r.state.Version++
			r.broadcastSnapshot()

		case c := <-r.leave:
			if _, ok := r.conns[c]; !ok {
				continue
			}
			r.lastActive = time.Now()
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
			if _, ok := r.conns[msg.c]; !ok {
				continue
			}
			r.lastActive = time.Now()
			r.process(msg)
		}
	}
}

func (r *Room) handleEvent(c transport.Client, typ string, raw json.RawMessage) *game.AppError {
	evType, payload, err := ParseClientEvent(typ, raw)
	if err != nil {
		slog.Warn(err.Error())

		return err
	}
	res, err := game.Reduce(r.state, c.UID(), evType, payload)
	if err != nil {
		slog.Warn(err.Error())
		return err
	}
	r.notice = res.Notice

	if res.Changed {
		r.state = res.State
		for i := range r.state.Seats {
			if r.state.Seats[i].UID == c.UID() {
				r.state.Seats[i].Name = clientName(c)
			}
		}
	}
	return nil
}

func (r *Room) broadcastSnapshot() {
	for c := range r.conns {
		view := game.MakeView(r.state, c.UID())
		snap := game.Snapshot{Type: "snapshot", State: view}
		_ = c.SendJSON(snap)
	}
}
