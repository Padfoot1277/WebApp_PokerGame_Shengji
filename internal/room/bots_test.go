package room

import (
	"encoding/gob"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

func TestRoomUsesSameModelSearchAsArena(t *testing.T) {
	for _, mode := range []string{"search", "hybrid"} {
		t.Run(mode, func(t *testing.T) {
			checkRoomSearch(t, mode == "hybrid")
		})
	}
}

func checkRoomSearch(t *testing.T, hybrid bool) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	model := bot.NewLearnModel(19)
	raw, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	model, err = bot.LoadLearnedPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRoom("learned")
	r.botModel = model
	r.botHybrid = hybrid
	r.state.Phase = game.PhasePlayTrick
	r.state.CallerSeat = 0
	r.state.Trump = game.TrumpState{Trump: rules.Trump{LevelRank: rules.R2}}
	r.state.Trick = game.TrickState{TurnSeat: 0, LeaderSeat: 0, BiggerSeat: -1}
	deck := rules.NewDoubleDeck()
	for i, uid := range []string{"bot:first", "one", "two", "three"} {
		hand := deck[4*i : 4*i+4]
		for j := range hand {
			hand[j].SuitClass = rules.ComputeSuitClass(hand[j], r.state.Trump.Trump)
		}
		r.state.Seats[i] = game.SeatState{UID: uid, Team: game.TeamOfSeat(i), Hand: hand, HandCount: len(hand), IsBot: i == 0}
	}
	r.botNext = time.Time{}
	r.conns[&fakeClient{uid: "one"}] = struct{}{}
	infer := bot.LearnedPlay
	if hybrid {
		infer = bot.HybridPlay
	}
	want, err := infer(r.state, model)
	if err != nil {
		t.Fatal(err)
	}
	r.scheduleBot(time.Now())
	select {
	case result := <-r.botResults:
		if result.err != nil || !reflect.DeepEqual(result.action, want) {
			t.Fatalf("worker returned %+v, want %+v", result, want)
		}
		before := r.state.Version
		r.applyBot(result)
		if r.state.Version == before || r.state.Seats[0].HandCount >= 4 {
			t.Fatal("learned move was not committed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("learned move did not arrive")
	}
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if invalid, err := bot.LoadLearnedPolicy(path); err == nil || invalid != nil {
		t.Fatal("accepted empty checkpoint")
	}
}

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

func TestRuleRoomSavesAceForBottom(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("rule-ending")
	r.state.Phase = game.PhasePlayTrick
	r.state.Points = 60
	r.state.CallerSeat, r.state.BottomOwnerSeat = 0, 0
	r.state.Trick = game.TrickState{TurnSeat: 0, LeaderSeat: 0, BiggerSeat: -1}
	r.state.Trump = game.TrumpState{Trump: rules.Trump{LevelRank: rules.R2, Suit: rules.Heart, HasTrumpSuit: true}}
	deck := rules.NewDoubleDeck()
	r.state.Bottom = []rules.Card{deck[4], deck[17]}
	for i, h := range [4][2]int{{0, 11}, {1, 10}, {8, 9}, {6, 7}} {
		hand := []rules.Card{deck[h[0]], deck[h[1]]}
		for j := range hand {
			hand[j].SuitClass = rules.ComputeSuitClass(hand[j], r.state.Trump.Trump)
		}
		r.state.Seats[i] = game.SeatState{UID: []string{"bot:first", "one", "two", "three"}[i], Team: game.TeamOfSeat(i), Hand: hand, HandCount: 2, IsBot: i == 0}
		r.state.Teams[i%2].LevelRank = rules.R2
	}
	r.botNext = time.Time{}
	r.conns[&fakeClient{uid: "one"}] = struct{}{}
	r.scheduleBot(time.Now())
	select {
	case result := <-r.botResults:
		if result.err != nil {
			t.Fatal(result.err)
		}
		p, ok := result.action.Payload.(game.PlayCardsPayload)
		if !ok || len(p.CardIDs) != 1 || p.CardIDs[0] != 11 {
			t.Fatalf("expected low spade to preserve the ace, got %v", result.action)
		}
		r.applyBot(result)
		if r.state.Seats[0].HandCount != 1 {
			t.Fatal("rule play was not committed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no room rule decision")
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
	db, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id string
	if err := db.QueryRow("SELECT room FROM rooms").Scan(&id); err != nil || id != "recent" {
		t.Fatalf("remaining room = %q: %v", id, err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&n); err != nil || n != 1 {
		t.Fatalf("remaining rooms = %d: %v", n, err)
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
	f, err := os.Create(roomPath("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(f).Encode(r.savedSnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
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

func TestSavedBotNamesUseRobotWording(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("names")
	r.state.Seats[0] = game.SeatState{UID: "bot:old", IsBot: true, Name: "陪练1", BotOrder: 1}
	r.state.Seats[1] = game.SeatState{UID: "human", Name: "陪练爱好者", Team: 1}
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	restored := NewRoom("names")
	if restored.loadErr != nil || restored.state.Seats[0].Name != "机器人1" || restored.state.Seats[1].Name != "陪练爱好者" {
		t.Fatal("bot wording migration renamed a human or failed to rename a bot")
	}
}
