package room

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"upgrade-lan/internal/game"
)

// Gob preserves private-to-the-protocol fields (hands, bottom and used card IDs).
// Room IDs are hashed, never interpreted as paths. Files are not served by HTTP.
type savedRoom struct {
	PlaysMask uint8
	LastMask  uint8
	State     game.GameState
	Receipts  map[string][]receipt
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
func (r *Room) save() error {
	if err := os.MkdirAll(dataDir(), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dataDir(), "room-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	saved := savedRoom{State: r.state, Receipts: r.receipts}
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
	err = gob.NewEncoder(f).Encode(saved)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, roomPath(r.id))
}
func (r *Room) load() error {
	f, err := os.Open(roomPath(r.id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var saved savedRoom
	if err = gob.NewDecoder(f).Decode(&saved); err != nil {
		return err
	}
	if saved.State.RoomID != r.id {
		return errors.New("room identity mismatch")
	}
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
		r.state.Seats[i].Online = false
		r.state.Seats[i].Ready = false
	}
	return nil
}
