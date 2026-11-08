package room

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
)

func TestBotsFIFOAndPersistence(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("test")
	c := &fakeClient{uid: "human"}
	r.conns[c] = struct{}{}
	r.process(incoming{c: c, typ: "room.sit", raw: json.RawMessage(`{"seat":0}`), reqID: "sit"})
	for _, id := range []string{"a", "b", "c"} {
		r.process(incoming{c: c, typ: "room.add_bot", raw: json.RawMessage(`{}`), reqID: id})
	}
	if len(r.conns) != 1 {
		t.Fatal("bots must not consume connections")
	}
	first, second := r.state.Seats[1].UID, r.state.Seats[2].UID
	version := r.state.Version
	r.process(incoming{c: c, typ: "room.add_bot", reqID: "a"})
	if r.state.Version != version {
		t.Fatal("retry duplicated bot")
	}
	if err := r.changeBot(c, true); err == nil {
		t.Fatal("added bot to full room")
	}
	r.process(incoming{c: c, typ: "room.remove_bot", reqID: "remove"})
	if r.state.Seats[1].UID != "" || r.state.Seats[0].UID != "human" {
		t.Fatal("did not remove oldest bot only")
	}
	r.process(incoming{c: c, typ: "room.add_bot", reqID: "replacement"})
	restored := NewRoom("test")
	if restored.loadErr != nil {
		t.Fatal(restored.loadErr)
	}
	if err := restored.changeBot(c, false); err != nil {
		t.Fatal(err)
	}
	if restored.state.Seats[2].UID != "" || restored.state.Seats[1].UID == first || restored.state.Seats[3].UID == second {
		t.Fatal("FIFO lost across restart")
	}
	for _, s := range restored.state.Seats {
		if s.IsBot && !s.Online {
			t.Fatal("restored bot offline")
		}
	}
	restored.state.Phase = game.PhasePlayTrick
	if err := restored.changeBot(c, false); err == nil {
		t.Fatal("removed bot during play")
	}
}
func TestBotSchedulerAndStaleResult(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("test")
	c := &fakeClient{uid: "human"}
	if err := r.changeBot(c, true); err != nil {
		t.Fatal(err)
	}
	r.botNext = time.Time{}
	r.scheduleBot(time.Now())
	if r.botBusy {
		t.Fatal("bots should pause without humans connected")
	}
	r.conns[c] = struct{}{}
	signal := time.Now().Add(-23 * time.Hour)
	r.lastActive = signal
	r.scheduleBot(time.Now())
	if !r.botBusy {
		t.Fatal("bot was not scheduled")
	}
	select {
	case result := <-r.botResults:
		r.applyBot(result)
	case <-time.After(5 * time.Second):
		t.Fatal("bot timeout")
	}
	if !r.state.Seats[0].Ready {
		t.Fatal("bot did not prepare")
	}
	if !r.lastActive.Equal(signal) {
		t.Fatal("bot refreshed human activity")
	}
	version := r.state.Version
	stale := botResult{version: version - 1, uid: r.state.Seats[0].UID, action: bot.Action{Type: string(game.EvUnready), Payload: struct{}{}}}
	r.applyBot(stale)
	if !r.state.Seats[0].Ready || r.state.Version != version {
		t.Fatal("stale worker changed state")
	}
}
func TestBotCommandRollbackOnStorageFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	r := NewRoom("test")
	c := &fakeClient{uid: "human"}
	bad := filepath.Join(dir, "file")
	if err := os.WriteFile(bad, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", bad)
	r.process(incoming{c: c, typ: "room.add_bot", reqID: "add"})
	for _, s := range r.state.Seats {
		if s.UID != "" {
			t.Fatal("failed save left a bot")
		}
	}
}
func TestExpiredRoomStartsFresh(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("test")
	r.state.Seats[0] = game.SeatState{UID: "old", IsBot: true, BotOrder: 3}
	r.state.Teams[0].LevelRank = "K"
	r.lastActive = time.Now().Add(-roomTTL - time.Minute)
	r.receipts["old"] = []receipt{{ID: "past", OK: true}}
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	// A bot save just modified the file. Last human activity must still expire it.
	if !r.expired(time.Now()) {
		t.Fatal("expired room not recognized")
	}
	fresh := NewRoom("test")
	if fresh.loadErr != nil || fresh.state.Seats[0].UID != "" || len(fresh.receipts) != 0 || fresh.state.Teams[0].LevelRank != "2" {
		t.Fatal("expired game recovered old data")
	}
	if _, err := os.Stat(roomPath("test")); !os.IsNotExist(err) {
		t.Fatal("expired save remains")
	}
}
func TestSweepUnloadedRooms(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	old := NewRoom("old")
	old.lastActive = time.Now().Add(-25 * time.Hour)
	if err := old.save(); err != nil {
		t.Fatal(err)
	}
	recent := NewRoom("recent")
	if err := recent.save(); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	if err := m.CleanupExpired(time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(roomPath("old")); !os.IsNotExist(err) {
		t.Fatal("expired unloaded room retained")
	}
	if _, err := os.Stat(roomPath("recent")); err != nil {
		t.Fatal("recent room removed")
	}
}

func TestActiveExpiredRoomClosesAndErases(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("test")
	c := &fakeClient{uid: "old-human"}
	r.conns[c] = struct{}{}
	r.lastActive = time.Now().Add(-25 * time.Hour)
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	retired := false
	r.retire = func() { retired = true }
	go r.Run()
	select {
	case <-r.done:
	case <-time.After(3 * time.Second):
		t.Fatal("expired loop still running")
	}
	if !retired || !c.closed {
		t.Fatal("expired room kept manager entry or connection")
	}
	if _, err := os.Stat(roomPath("test")); !os.IsNotExist(err) {
		t.Fatal("expired save retained")
	}
}

func TestLegacySaveExpiresByModificationTime(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("legacy")
	r.state.Seats[0].UID = "old"
	r.lastActive = time.Time{}
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(roomPath("legacy"), old, old); err != nil {
		t.Fatal(err)
	}
	fresh := NewRoom("legacy")
	if fresh.loadErr != nil || fresh.state.Seats[0].UID != "" {
		t.Fatal("legacy expired data recovered")
	}
}
