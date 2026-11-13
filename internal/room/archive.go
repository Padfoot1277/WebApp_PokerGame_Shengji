package room

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/game/rules"
)

// SQLite's rollback journals commit attached databases together. The live
// database can be removed at deployment without touching the archive directory.
func openStore() (*sql.DB, error) {
	dir := filepath.Join(dataDir(), "archive")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := driver.Open(filepath.Join(dataDir(), "rooms.sqlite"), func(conn *sqlite3.Conn) error {
		stmt, _, err := conn.Prepare("ATTACH DATABASE ? AS archive")
		if err != nil {
			return err
		}
		defer stmt.Close()
		if err := stmt.BindText(1, filepath.Join(dir, "rounds.sqlite")); err != nil {
			return err
		}
		if err := stmt.Exec(); err != nil {
			return err
		}
		return conn.Exec("PRAGMA foreign_keys=ON")
	})
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS rooms (room TEXT PRIMARY KEY, signal INTEGER NOT NULL, snapshot BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS rooms_by_signal ON rooms(signal);
CREATE TABLE IF NOT EXISTS pending_rounds (
 id TEXT PRIMARY KEY, room TEXT NOT NULL UNIQUE REFERENCES rooms(room) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
 round_index INTEGER NOT NULL, from_deal INTEGER NOT NULL, initial_state TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS pending_actions (
 round_id TEXT NOT NULL REFERENCES pending_rounds(id) ON DELETE CASCADE,
 version INTEGER NOT NULL, seat INTEGER NOT NULL, event TEXT NOT NULL, payload TEXT NOT NULL,
 state_after TEXT NOT NULL, PRIMARY KEY(round_id, version)) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS archive.rounds (
 id TEXT PRIMARY KEY, room TEXT NOT NULL, round_index INTEGER NOT NULL,
 from_deal INTEGER NOT NULL, phase TEXT NOT NULL, initial_state TEXT NOT NULL,
 format_version INTEGER NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS archive.actions (
 round_id TEXT NOT NULL REFERENCES rounds(id), version INTEGER NOT NULL, seat INTEGER NOT NULL,
 event TEXT NOT NULL, payload TEXT NOT NULL, state_after TEXT NOT NULL,
 PRIMARY KEY(round_id, version)) WITHOUT ROWID;`)
	if err == nil {
		var schema int
		err = db.QueryRow("PRAGMA archive.user_version").Scan(&schema)
		if err == nil && schema == 0 {
			err = migrateUnsettled(db)
		}
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Older archives included unfinished rounds even after a room was reused.
// Only the ID in a live snapshot identifies a round that can still resume.
func migrateUnsettled(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT snapshot FROM rooms WHERE room IN
        (SELECT room FROM archive.rounds WHERE phase NOT IN ('round_settle','game_over'))`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var snapshot []byte
		var saved struct{ RoundID string }
		if err := rows.Scan(&snapshot); err != nil {
			rows.Close()
			return err
		}
		if err := gob.NewDecoder(bytes.NewReader(snapshot)).Decode(&saved); err != nil {
			rows.Close()
			return err
		}
		if saved.RoundID != "" {
			ids = append(ids, saved.RoundID)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		if _, err = tx.Exec(`INSERT INTO pending_rounds(id,room,round_index,from_deal,initial_state)
            SELECT id,room,round_index,from_deal,initial_state FROM archive.rounds
            WHERE id=? AND phase NOT IN ('round_settle','game_over')`, id); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO pending_actions SELECT a.* FROM archive.actions a JOIN pending_rounds p ON p.id=a.round_id;
        DELETE FROM archive.actions WHERE round_id IN (SELECT id FROM archive.rounds WHERE phase NOT IN ('round_settle','game_over'));
        DELETE FROM archive.rounds WHERE phase NOT IN ('round_settle','game_over');
        PRAGMA archive.user_version=1;`); err != nil {
		return err
	}
	return tx.Commit()
}

// The public JSON protocol omits private fields. Include them only here.
func archiveState(s game.GameState) (string, error) {
	var hands [4][]rules.Card
	for i, seat := range s.Seats {
		hands[i] = seat.Hand
	}
	b, err := json.Marshal(struct {
		game.GameState
		Hands           [4][]rules.Card `json:"hands"`
		Bottom          []rules.Card    `json:"bottom"`
		UsedTrumpIDs    map[int]bool    `json:"usedTrumpIds"`
		SurrenderMask   uint8           `json:"surrenderMask"`
		NextStarterSeat int             `json:"nextStarterSeat"`
		CallPassMask    uint8           `json:"callPassMask"`
		FightPassMask   uint8           `json:"fightPassMask"`
	}{s, hands, s.Bottom, s.UsedTrumpIDs, s.SurrenderMask, s.NextStarterSeat, s.CallPassMask, s.FightPassMask})
	return string(b), err
}

type roundAction struct {
	before     game.GameState
	uid, event string
	payload    any
}

func startPendingRound(tx *sql.Tx, s game.GameState, fromDeal bool) (string, error) {
	initial, err := archiveState(s)
	if err != nil {
		return "", err
	}
	id := rand.Text()
	_, err = tx.Exec("INSERT INTO pending_rounds(id,room,round_index,from_deal,initial_state) VALUES(?,?,?,?,?)", id, s.RoomID, s.RoundIndex, fromDeal, initial)
	return id, err
}

func (r *Room) persist(action *roundAction) error {
	db, err := openStore()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id := r.roundID
	if action != nil && action.before.RoundIndex != r.state.RoundIndex {
		id = ""
	}
	if id == "" && r.state.Phase != game.PhaseLobby {
		initial := r.state
		fromDeal := action != nil && (action.before.Phase == game.PhaseLobby || action.before.RoundIndex != r.state.RoundIndex)
		if action != nil && !fromDeal {
			initial = action.before
		}
		id, err = startPendingRound(tx, initial, fromDeal)
		if err != nil {
			return err
		}
	}
	if action != nil && id != "" {
		seat := -1
		for i, s := range r.state.Seats {
			if s.UID == action.uid {
				seat = i
				break
			}
		}
		payload, err := json.Marshal(action.payload)
		if err != nil {
			return err
		}
		after, err := archiveState(r.state)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO pending_actions VALUES(?,?,?,?,?,?)", id, r.state.Version, seat, action.event, string(payload), after); err != nil {
			return err
		}
	}
	// Connection saves after settlement already have a published round ID.
	if (r.state.Phase == game.PhaseRoundSettle || r.state.Phase == game.PhaseGameOver) && (action != nil || r.roundID == "") {
		if _, err = tx.Exec(`INSERT INTO archive.rounds(id,room,round_index,from_deal,phase,initial_state)
            SELECT id,room,round_index,from_deal,?,initial_state FROM pending_rounds WHERE id=?`, r.state.Phase, id); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO archive.actions SELECT * FROM pending_actions WHERE round_id=?`, id); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM pending_rounds WHERE id=?", id); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	saved := r.savedSnapshot()
	saved.RoundID = id
	if err = gob.NewEncoder(&buf).Encode(saved); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO rooms(room,signal,snapshot) VALUES(?,?,?) ON CONFLICT(room) DO UPDATE SET signal=excluded.signal,snapshot=excluded.snapshot", r.id, r.lastActive.Unix(), buf.Bytes()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	r.roundID = id
	return nil
}

func (r *Room) load() error {
	db, err := openStore()
	if err != nil {
		return err
	}
	defer db.Close()
	var raw []byte
	err = db.QueryRow("SELECT snapshot FROM rooms WHERE room=?", r.id).Scan(&raw)
	var saved savedRoom
	if errors.Is(err, sql.ErrNoRows) {
		// Pre-database installations kept one gob per room.
		f, err := os.Open(roomPath(r.id))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer f.Close()
		if err = gob.NewDecoder(f).Decode(&saved); err != nil {
			return err
		}
		if saved.LastSignal.IsZero() {
			info, err := f.Stat()
			if err != nil {
				return err
			}
			saved.LastSignal = info.ModTime()
		}
		f.Close()
	} else if err != nil {
		return err
	} else if err = gob.NewDecoder(bytes.NewReader(raw)).Decode(&saved); err != nil {
		return err
	}
	if saved.State.RoomID != r.id {
		return errors.New("room identity mismatch")
	}
	if raw == nil {
		// Import into a temporary room so an expired snapshot can contribute
		// its remaining evidence without reviving the retired seats.
		legacy := Room{id: r.id}
		legacy.restore(saved)
		if err := legacy.save(); err != nil {
			return err
		}
		saved.RoundID = legacy.roundID
	}
	if time.Since(saved.LastSignal) >= roomTTL {
		return r.erase()
	}
	r.restore(saved)
	// Also finish removal after a crash between import commit and unlink.
	if err := os.Remove(roomPath(r.id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
