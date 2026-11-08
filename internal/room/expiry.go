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
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
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
		info, statErr := f.Stat()
		var saved savedRoom
		decodeErr := gob.NewDecoder(f).Decode(&saved)
		_ = f.Close()
		if statErr != nil {
			return statErr
		}
		signal := saved.LastSignal
		if signal.IsZero() {
			signal = info.ModTime()
		}
		if decodeErr != nil {
			signal = info.ModTime()
		}
		if now.Sub(signal) >= roomTTL {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
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
