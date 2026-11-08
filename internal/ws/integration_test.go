package ws_test

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/room"
	"upgrade-lan/internal/ws"
)

type peer struct {
	socket     *websocket.Conn
	mu         sync.Mutex
	view       game.ViewState
	uid, token string
	acks       chan map[string]any
	done       chan struct{}
}

func connect(t *testing.T, url string) *peer {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := &peer{socket: c, acks: make(chan map[string]any, 8), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		for {
			_, raw, err := c.ReadMessage()
			if err != nil {
				return
			}
			var envelope struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(raw, &envelope)
			switch envelope.Type {
			case "hello":
				var msg struct{ UID, Token string }
				_ = json.Unmarshal(raw, &msg)
				p.mu.Lock()
				p.uid = msg.UID
				p.token = msg.Token
				p.mu.Unlock()
			case "snapshot":
				var msg game.Snapshot
				_ = json.Unmarshal(raw, &msg)
				p.mu.Lock()
				p.view = msg.State
				p.mu.Unlock()
			case "ack":
				var msg map[string]any
				_ = json.Unmarshal(raw, &msg)
				p.acks <- msg
			}
		}
	}()
	return p
}
func (p *peer) snapshot() game.ViewState { p.mu.Lock(); defer p.mu.Unlock(); return p.view }
func (p *peer) send(t *testing.T, id, typ string, payload any) {
	t.Helper()
	if err := p.socket.WriteJSON(map[string]any{"type": typ, "payload": payload, "reqId": id}); err != nil {
		t.Fatal(err)
	}
	select {
	case ack := <-p.acks:
		if ack["reqId"] != id || ack["ok"] != true {
			t.Fatalf("request %s failed: %v", typ, ack)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ack timed out")
	}
}
func wait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatal("snapshot timed out")
}
func TestFourPlayersFullRoundAndReconnect(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	hub := ws.NewHub()
	go hub.Run()
	manager := room.NewManager()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ws.ServeWS(hub, manager, w, r) }))
	defer server.Close()
	base := "ws" + strings.TrimPrefix(server.URL, "http") + "?room=integration&name=same"
	players := make([]*peer, 4)
	for i := range players {
		players[i] = connect(t, base)
		defer func(i int) { _ = players[i].socket.Close() }(i)
		p := players[i]
		wait(t, func() bool { return p.snapshot().RoomID != "" })
		p.send(t, fmt.Sprintf("sit-%d", i), "room.sit", map[string]int{"seat": i})
	}
	if players[0].uid == players[1].uid {
		t.Fatal("same nicknames share identity")
	}
	for i, p := range players {
		p.send(t, fmt.Sprintf("ready-%d", i), "room.ready", struct{}{})
	}
	wait(t, func() bool { return players[0].snapshot().Phase == game.PhaseCallTrump })
	for i, p := range players {
		wait(t, func() bool { return p.snapshot().Phase == game.PhaseCallTrump })
		p.send(t, fmt.Sprintf("pass-%d", i), "game.call_pass", struct{}{})
	}
	wait(t, func() bool { return players[0].snapshot().Phase == game.PhasePlayTrick })
	// Refresh a seated player using its server-issued token.
	token := players[2].token
	_ = players[2].socket.Close()
	players[2] = connect(t, base+"&token="+token)
	wait(t, func() bool { return players[2].snapshot().MySeat == 2 })
	sequence := 0
	for players[0].snapshot().Phase == game.PhasePlayTrick {
		view := players[0].snapshot()
		seat := view.Trick.TurnSeat
		p := players[seat]
		wait(t, func() bool { return p.snapshot().Version >= view.Version })
		mine := p.snapshot()
		var chosen int
		found := false
		lead := mine.Trick.Plays[mine.Trick.LeaderSeat]
		for _, group := range mine.MyHand {
			for _, card := range group {
				if !found {
					chosen = card.ID
				}
				if lead == nil || card.SuitClass == lead.SuitClass {
					chosen = card.ID
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		sequence++
		p.send(t, fmt.Sprintf("play-%d", sequence), "game.play_cards", map[string]any{"cardIds": []int{chosen}})
		wait(t, func() bool { return players[0].snapshot().Version > view.Version })
		if sequence > 100 {
			t.Fatal("round did not end")
		}
	}
	end := players[0].snapshot()
	if sequence != 100 || end.Phase != game.PhaseRoundSettle || !end.BottomRevealed {
		t.Fatalf("bad end: plays=%d phase=%s", sequence, end.Phase)
	}
	starter := end.NextStarterSeat
	players[starter].send(t, "next", "game.start_next_round", struct{}{})
	wait(t, func() bool { return players[0].snapshot().Phase == game.PhaseCallTrump })
	if players[0].snapshot().BottomAward != 0 {
		t.Fatal("stale bottom award")
	}
}

func TestMalformedMessagesCannotBypassRateLimit(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	hub := ws.NewHub()
	go hub.Run()
	manager := room.NewManager()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ws.ServeWS(hub, manager, w, r) }))
	defer server.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?room=bad-json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// Ensure the initial handshake completed before starting the burst.
	for i := 0; i < 2; i++ {
		if _, _, err = c.ReadMessage(); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 31; i++ {
		if err = c.WriteMessage(websocket.TextMessage, []byte("{")); err != nil {
			break
		}
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		if _, _, err = c.ReadMessage(); err != nil {
			if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
				t.Fatal("malformed messages bypassed the rate limit")
			}
			break
		}
	}
}

func TestRoomBotsThroughRealWebSocket(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	hub := ws.NewHub()
	go hub.Run()
	manager := room.NewManager()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ws.ServeWS(hub, manager, w, r) }))
	defer server.Close()
	p := connect(t, "ws"+strings.TrimPrefix(server.URL, "http")+"?room=with-bots&name=human")
	defer p.socket.Close()
	wait(t, func() bool { return p.snapshot().RoomID != "" })
	p.send(t, "sit", "room.sit", map[string]int{"seat": 0})
	for _, id := range []string{"bot1", "bot2", "bot3"} {
		p.send(t, id, "room.add_bot", struct{}{})
	}
	wait(t, func() bool {
		v := p.snapshot()
		return v.Seats[1].IsBot && v.Seats[1].Ready && v.Seats[2].Ready && v.Seats[3].Ready
	})
	v := p.snapshot()
	first, second := v.Seats[1].UID, v.Seats[2].UID
	if len(v.MyHand) != 0 {
		t.Fatal("lobby exposed a hand")
	}
	p.send(t, "remove", "room.remove_bot", struct{}{})
	wait(t, func() bool { return p.snapshot().Seats[1].UID == "" })
	p.send(t, "replace", "room.add_bot", struct{}{})
	wait(t, func() bool {
		v := p.snapshot()
		return v.Seats[1].Ready && v.Seats[1].UID != first && v.Seats[2].UID == second
	})
	p.send(t, "ready", "room.ready", struct{}{})
	wait(t, func() bool { return p.snapshot().Phase != game.PhaseLobby })
	v = p.snapshot()
	if len(v.MyHand) == 0 {
		t.Fatal("human did not receive own hand")
	}
	for _, seat := range v.Seats {
		if seat.IsBot && seat.HandCount != 25 {
			t.Fatal("bot did not receive cards")
		}
	}
}
