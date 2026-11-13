package room

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
	"upgrade-lan/internal/game"
)

// Gob preserves private-to-the-protocol fields (hands, bottom and used card IDs).
// Room IDs are hashed, never interpreted as paths. Files are not served by HTTP.
type savedRoom struct {
	RoundID    string
	LastSignal time.Time
	PlaysMask  uint8
	LastMask   uint8
	State      game.GameState
	Receipts   map[string][]receipt
}

func dataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return "data"
}
func roomPath(id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(dataDir(), hex.EncodeToString(sum[:])+".gob")
}
func (r *Room) save() error { return r.persist(nil) }

func (r *Room) savedSnapshot() savedRoom {
	saved := savedRoom{State: r.state, Receipts: r.receipts, LastSignal: r.lastActive}
	for i := 0; i < 4; i++ {
		if saved.State.Trick.Plays[i] != nil {
			saved.PlaysMask |= 1 << uint(i)
		} else {
			saved.State.Trick.Plays[i] = &game.PlayedMove{}
		}
		if saved.State.Trick.LastPlays[i] != nil {
			saved.LastMask |= 1 << uint(i)
		} else {
			saved.State.Trick.LastPlays[i] = &game.PlayedMove{}
		}
	}
	return saved
}

func (r *Room) restore(saved savedRoom) {
	r.roundID = saved.RoundID
	r.lastActive = saved.LastSignal
	for i := 0; i < 4; i++ {
		if saved.PlaysMask&(1<<uint(i)) == 0 {
			saved.State.Trick.Plays[i] = nil
		}
		if saved.LastMask&(1<<uint(i)) == 0 {
			saved.State.Trick.LastPlays[i] = nil
		}
	}
	r.state = saved.State
	r.receipts = saved.Receipts
	if r.receipts == nil {
		r.receipts = make(map[string][]receipt)
	}
	for i := range r.state.Seats {
		if r.state.Seats[i].IsBot {
			r.state.Seats[i].Name = strings.ReplaceAll(r.state.Seats[i].Name, "陪练", "机器人")
		}
		r.state.Seats[i].Online = r.state.Seats[i].IsBot
		r.state.Seats[i].Ready = false
	}
}
