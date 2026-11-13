package room

import (
	"encoding/gob"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"upgrade-lan/internal/game"
)

const roomTTL = 24 * time.Hour

func (r *Room) expired(now time.Time) bool {
	return !r.lastActive.IsZero() && now.Sub(r.lastActive) >= roomTTL
}
func (r *Room) erase() error {
	err := os.Remove(roomPath(r.id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	db, err := openStore()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("DELETE FROM rooms WHERE room=?", r.id)
	return err
}

// Loaded rooms expire on their own event loop. This sweep covers saved rooms
// which have not been loaded since a restart, without refreshing their age.
func (m *Manager) CleanupExpired(now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(dataDir(), "*.gob"))
	if err != nil {
		return err
	}
	active := map[string]bool{}
	for id := range m.rooms {
		active[roomPath(id)] = true
	}
	for _, path := range paths {
		if active[path] {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		var saved savedRoom
		decodeErr := gob.NewDecoder(f).Decode(&saved)
		info, statErr := f.Stat()
		f.Close()
		if decodeErr != nil {
			if statErr != nil {
				return statErr
			}
			if now.Sub(info.ModTime()) >= roomTTL {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
			continue
		}
		if roomPath(saved.State.RoomID) != path {
			return errors.New("legacy room identity mismatch")
		}
		if r := NewRoom(saved.State.RoomID); r.loadErr != nil {
			return r.loadErr
		}
	}
	db, err := openStore()
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query("SELECT room FROM rooms WHERE signal <= ?", now.Add(-roomTTL).Unix())
	if err != nil {
		return err
	}
	var expired []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if m.rooms[id] == nil {
			expired = append(expired, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range expired {
		if _, err := db.Exec("DELETE FROM rooms WHERE room=? AND signal <= ?", id, now.Add(-roomTTL).Unix()); err != nil {
			return err
		}
	}
	return nil
}

func (r *Room) expireRoom(now time.Time) bool {
	if !r.expired(now) {
		return false
	}
	if err := r.erase(); err != nil {
		slog.Error("room expiration failed", "room", r.id, "error", err)
		return false
	}
	if r.retire != nil {
		r.retire()
	}
	for c := range r.conns {
		_ = c.SendJSON(game.NoticeMsg{Type: "notice", Message: "房间已超过24小时无新操作，牌局已清除，请重新进入"})
		_ = c.Close()
	}
	return true
}
