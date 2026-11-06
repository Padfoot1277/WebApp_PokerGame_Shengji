package room

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"upgrade-lan/internal/game"
	"upgrade-lan/internal/transport"
)

type receipt struct {
	ID      string
	OK      bool
	Code    string
	Message string
}

func clientName(c transport.Client) string {
	if v, ok := c.(interface{ Name() string }); ok {
		return v.Name()
	}
	return "牌友"
}
func clientToken(c transport.Client) string {
	if v, ok := c.(interface{ Token() string }); ok {
		return v.Token()
	}
	return ""
}
func (r *Room) process(msg incoming) {
	before := r.state
	r.notice = ""
	var result receipt
	for _, old := range r.receipts[msg.c.UID()] {
		if old.ID == msg.reqID {
			r.sendReceipt(msg.c, old)
			r.sendSnapshot(msg.c)
			return
		}
	}
	result = receipt{ID: msg.reqID, OK: true}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("room command panic", "room", r.id, "event", msg.typ, "panic", recovered)
				result.OK = false
				result.Code = "SYS_INTERNAL_ERROR"
				result.Message = "操作失败，请重试"
			}
		}()
		if err := r.handleEvent(msg.c, msg.typ, msg.raw); err != nil {
			result.OK = false
			result.Code = err.Code
			result.Message = err.Error()
		}
	}()
	history := append(r.receipts[msg.c.UID()], result)
	if len(history) > 128 {
		history = history[len(history)-128:]
	}
	old := r.receipts[msg.c.UID()]
	r.receipts[msg.c.UID()] = history
	if err := r.save(); err != nil {
		r.state = before
		r.receipts[msg.c.UID()] = old
		r.notice = ""
		result.OK = false
		result.Code = "STORAGE_UNAVAILABLE"
		result.Message = "保存牌局失败，本次操作未生效"
		slog.Error("room save failed", "room", r.id, "error", err)
	} else {
		if r.notice != "" {
			for c := range r.conns {
				_ = c.SendJSON(game.NoticeMsg{Type: "notice", Message: r.notice})
			}
		}
		r.broadcastSnapshot()
		r.logCommand(msg, result)
	}
	r.sendReceipt(msg.c, result)
}
func (r *Room) sendReceipt(c transport.Client, v receipt) {
	_ = c.SendJSON(map[string]any{"type": "ack", "reqId": v.ID, "ok": v.OK, "code": v.Code, "message": v.Message, "version": r.state.Version})
}
func (r *Room) sendSnapshot(c transport.Client) {
	_ = c.SendJSON(game.Snapshot{Type: "snapshot", State: game.MakeView(r.state, c.UID())})
}

func (r *Room) logCommand(msg incoming, result receipt) {
	f, err := os.OpenFile(filepath.Join(dataDir(), "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		slog.Warn("event log unavailable", "error", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(map[string]any{"time": time.Now().UTC(), "room": r.id, "uid": msg.c.UID(), "reqId": msg.reqID, "event": msg.typ, "payload": msg.raw, "ok": result.OK, "code": result.Code, "version": r.state.Version})
}
