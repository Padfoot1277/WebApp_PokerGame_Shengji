package room

import (
	"encoding/json"
	"testing"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

type fakeClient struct {
	uid      string
	messages []any
	closed   bool
}

func (c *fakeClient) UID() string          { return c.uid }
func (c *fakeClient) RoomID() string       { return "test" }
func (c *fakeClient) SendJSON(v any) error { c.messages = append(c.messages, v); return nil }
func (c *fakeClient) Close() error         { c.closed = true; return nil }
func TestIdempotencyAndPersistence(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("test")
	c := &fakeClient{uid: "one"}
	r.conns[c] = struct{}{}
	msg := incoming{c: c, typ: "room.sit", raw: json.RawMessage(`{"seat":1}`), reqID: "request-1"}
	r.process(msg)
	version := r.state.Version
	r.process(msg)
	if r.state.Version != version {
		t.Fatal("duplicate command applied twice")
	}
	restored := NewRoom("test")
	if restored.loadErr != nil || restored.state.Seats[1].UID != "one" || restored.state.Seats[1].Online {
		t.Fatal("room failed to recover")
	}
	restored.conns[c] = struct{}{}
	restored.process(msg)
	if restored.state.Version != version {
		t.Fatal("receipt not restored")
	}
	restored.state.Phase = game.PhasePlayTrick
	restored.state.Seats[1].Hand = []rules.Card{{ID: 45}}
	restored.state.Bottom = []rules.Card{{ID: 98}}
	restored.state.UsedTrumpIDs = map[int]bool{45: true}
	if err := restored.save(); err != nil {
		t.Fatal(err)
	}
	again := NewRoom("test")
	if len(again.state.Seats[1].Hand) != 1 || len(again.state.Bottom) != 1 || !again.state.UsedTrumpIDs[45] {
		t.Fatal("private game fields lost")
	}
}
func TestMalformedPayload(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `{"seat":-1}`, `{"seat":4}`, `{"seat":"east"}`} {
		if _, _, err := ParseClientEvent("room.sit", json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

type lifecycleClient struct {
	fakeClient
	done chan struct{}
}

func (c *lifecycleClient) Done() <-chan struct{} { return c.done }

func TestReplacedConnectionCannotRejoinAfterReplacement(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("test")
	current := &lifecycleClient{fakeClient: fakeClient{uid: "same"}, done: make(chan struct{})}
	r.acceptConnection(current)
	r.process(incoming{c: current, typ: "room.sit", raw: json.RawMessage(`{"seat":0}`), reqID: "sit"})
	old := &lifecycleClient{fakeClient: fakeClient{uid: "same"}, done: make(chan struct{})}
	close(old.done)
	version := r.state.Version
	// The old HTTP handler can resume its join after the new connection joined.
	r.acceptConnection(old)
	if current.closed || len(r.conns) != 1 || r.state.Version != version {
		t.Fatal("late closed connection displaced current player")
	}
	if _, ok := r.conns[current]; !ok || !r.state.Seats[0].Online {
		t.Fatal("current player lost its online seat")
	}
}

func TestRecordOptionPersistsWithRoom(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	for _, enabled := range []bool{false, true} {
		id := "record-off"
		if enabled {
			id = "record-on"
		}
		r := NewRoom(id, enabled)
		if r.state.HideRecord == enabled {
			t.Fatal("incorrect room record option")
		}
		if err := r.save(); err != nil {
			t.Fatal(err)
		}
		restored := NewRoom(id, !enabled)
		if restored.state.HideRecord != r.state.HideRecord {
			t.Fatal("reconnect changed room option")
		}
	}
}
