package room

import (
	"encoding/gob"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/game"
)

func TestRoundArchiveSurvivesRestartRetriesAndRoomRemoval(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	c := &fakeClient{uid: "player"}
	deal := func() *Room {
		r := NewRoom("reused")
		r.process(incoming{c, "room.sit", json.RawMessage(`{"seat":0}`), "sit"})
		for i := 0; i < 3; i++ {
			if err := r.changeBot(c, true); err != nil {
				t.Fatal(err)
			}
			r.state.Seats[i+1].Ready = true
		}
		r.process(incoming{c, "room.ready", nil, "ready"})
		if r.state.Phase != game.PhaseCallTrump || r.roundID == "" {
			t.Fatal("deal missing")
		}
		return r
	}
	r := deal()
	id := r.roundID
	r.applyBot(botResult{version: r.state.Version, uid: r.state.Seats[1].UID, action: bot.Action{Type: "game.call_pass", Payload: struct{}{}}})
	r = NewRoom(r.id)
	if r.loadErr != nil || r.roundID != id {
		t.Fatalf("resume: %v", r.loadErr)
	}
	pass := incoming{c, "game.call_pass", nil, "pass"}
	db, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TRIGGER fail_insert BEFORE INSERT ON pending_actions BEGIN SELECT RAISE(ABORT,'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	version := r.state.Version
	r.process(pass)
	if r.state.Version != version || NewRoom(r.id).state.Version != version {
		t.Fatal("archive failure committed room")
	}
	if _, err := db.Exec("DROP TRIGGER fail_insert"); err != nil {
		t.Fatal(err)
	}
	r.process(pass)
	r.process(pass)
	if r.state.Version != version+1 {
		t.Fatal("retry not committed exactly once")
	}
	// The partially played round remains resumable but absent from training data.
	var n int
	if err := db.QueryRow("SELECT count(*) FROM archive.rounds").Scan(&n); err != nil || n != 0 {
		t.Fatalf("premature archives=%d: %v", n, err)
	}
	for _, seat := range []int{2, 3} {
		r.applyBot(botResult{version: r.state.Version, uid: r.state.Seats[seat].UID, action: bot.Action{Type: "game.call_pass", Payload: struct{}{}}})
	}
	r.process(incoming{c, "game.surrender", nil, "surrender"})
	r.applyBot(botResult{version: r.state.Version, uid: r.state.Seats[2].UID, action: bot.Action{Type: "game.surrender", Payload: struct{}{}}})
	if r.state.Phase != game.PhaseRoundSettle || r.state.RoundPointsFinal != 0 {
		t.Fatalf("expected settled zero-point round: %s", r.state.Phase)
	}
	if err := NewManager().CleanupExpired(time.Now().Add(25 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if fresh := NewRoom(r.id); fresh.state.Phase != game.PhaseLobby || fresh.roundID != "" {
		t.Fatal("room did not expire")
	}
	next := deal()
	if id == next.roundID {
		t.Fatal("room reuse overwrote history")
	}
	// Simulate clearing live storage during offline deployment.
	db.Close()
	if err := os.Remove(filepath.Join(dir, "rooms.sqlite")); err != nil {
		t.Fatal(err)
	}
	db, err = openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow("SELECT count(*) FROM archive.rounds").Scan(&n); err != nil || n != 1 {
		t.Fatalf("rounds=%d: %v", n, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM archive.actions WHERE round_id=? AND event='game.call_pass' AND ((seat=0 AND json_extract(state_after,'$.seats[0].isBot')=0) OR (seat=1 AND json_extract(state_after,'$.seats[1].isBot')=1))`, id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("passes=%d: %v", n, err)
	}
	var initial struct {
		Hands  [4][]struct{ ID int }
		Bottom []struct{ ID int }
	}
	var raw string
	if err := db.QueryRow("SELECT initial_state FROM archive.rounds WHERE id=? AND from_deal=1", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &initial); err != nil {
		t.Fatal(err)
	}
	cards := map[int]bool{}
	for _, hand := range initial.Hands {
		if len(hand) != 25 {
			t.Fatal("missing hand")
		}
		for _, card := range hand {
			cards[card.ID] = true
		}
	}
	for _, card := range initial.Bottom {
		cards[card.ID] = true
	}
	if len(initial.Bottom) != 8 || len(cards) != 108 {
		t.Fatal("incomplete deal")
	}
}

func TestUnsettledLegacyRoomExpiresWithoutArchive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	r := NewRoom("legacy")
	r.state.Phase = game.PhasePlayTrick
	r.state.TrickIndex = 4
	f, err := os.Create(roomPath(r.id))
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(f).Encode(r.savedSnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	if err := m.CleanupExpired(time.Now()); err != nil {
		t.Fatal(err)
	}
	recovered := NewRoom(r.id)
	if recovered.loadErr != nil || recovered.roundID == "" || recovered.state.TrickIndex != 4 {
		t.Fatalf("legacy recovery: %v", recovered.loadErr)
	}
	if err := m.CleanupExpired(time.Now().Add(25 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	db, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM archive.rounds WHERE from_deal=0 AND json_extract(initial_state,'$.trickIndex')=4`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("legacy archives=%d: %v", n, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM pending_rounds").Scan(&n); err != nil || n != 0 {
		t.Fatalf("expired drafts=%d: %v", n, err)
	}

}
func TestMigrationRetainsOnlySettledArchiveAndResumableDraft(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	r := NewRoom("table")
	r.state.Phase = game.PhasePlayTrick
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	db, err := openStore()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Recreate the previous layout: both the current round and a discarded
	// round with the same room code lived alongside a completed round.
	_, err = db.Exec(`
        INSERT INTO archive.rounds(id,room,round_index,from_deal,phase,initial_state)
            SELECT id,room,round_index,from_deal,'play_trick',initial_state FROM pending_rounds;
        DELETE FROM pending_rounds;
        INSERT INTO archive.rounds VALUES ('abandoned','table',0,1,'bottom','{}',1),('finished','table',0,1,'game_over','{}',1);
        INSERT INTO archive.actions SELECT id,1,0,'game.play_cards','{}','{}' FROM archive.rounds;
        PRAGMA archive.user_version=0;`)
	if err != nil {
		t.Fatal(err)
	}
	recovered := NewRoom("table")
	if recovered.loadErr != nil || recovered.roundID != r.roundID {
		t.Fatalf("migration: %v", recovered.loadErr)
	}
	var finished, active string
	var archived, drafts int
	if err := db.QueryRow("SELECT count(*),min(round_id) FROM archive.actions").Scan(&archived, &finished); err != nil || archived != 1 || finished != "finished" {
		t.Fatalf("archived=%d id=%q: %v", archived, finished, err)
	}
	if err := db.QueryRow("SELECT count(*),min(round_id) FROM pending_actions").Scan(&drafts, &active); err != nil || drafts != 1 || active != r.roundID {
		t.Fatalf("drafts=%d id=%q: %v", drafts, active, err)
	}
	if err := recovered.erase(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM pending_actions").Scan(&drafts); err != nil || drafts != 0 {
		t.Fatalf("remaining draft actions=%d: %v", drafts, err)
	}
}
