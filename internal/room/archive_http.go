package room

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
)

type archiveEntry struct {
	ID          string `json:"id"`
	Room        string `json:"room"`
	Number      int    `json:"number"`
	Points      int    `json:"points"`
	Result      string `json:"result"`
	Surrendered bool   `json:"surrendered"`
}

const settled = "phase IN ('round_settle','game_over')"

func RegisterArchive(mux *http.ServeMux) {
	absolute, err := filepath.Abs(filepath.Join(dataDir(), "archive", "rounds.sqlite"))
	if err != nil {
		panic(err)
	}
	uriPath := filepath.ToSlash(absolute)
	escaped := (&url.URL{Path: uriPath}).EscapedPath()
	db, err := sql.Open("sqlite3", "file:"+escaped+"?mode=ro")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(4)
	mux.HandleFunc("GET /api/rounds", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cursor := int64(0)
		if q.Get("before") != "" {
			n, err := strconv.ParseInt(q.Get("before"), 10, 64)
			if err != nil || n <= 0 {
				http.Error(w, "无效的页码", 400)
				return
			}
			cursor = n
		}
		rows, err := db.QueryContext(r.Context(), `SELECT r.rowid,r.id,r.room,r.round_index+1,
  json_extract(COALESCE(a.state_after,r.initial_state),'$.roundPointsFinal'),
  json_extract(COALESCE(a.state_after,r.initial_state),'$.roundResultLabel'),
  json_extract(COALESCE(a.state_after,r.initial_state),'$.surrendered')
  FROM rounds r LEFT JOIN actions a ON a.round_id=r.id AND a.version=(SELECT max(version) FROM actions WHERE round_id=r.id)
  WHERE `+settled+` AND (?='' OR r.room=?) AND (?=0 OR r.rowid<?)
  ORDER BY r.rowid DESC LIMIT 21`, q.Get("room"), q.Get("room"), cursor, cursor)
		if err != nil {
			archiveError(w, err)
			return
		}
		defer rows.Close()
		entries := []archiveEntry{}
		var last, next int64
		for rows.Next() {
			if len(entries) == 20 {
				next = last
				break
			}
			var e archiveEntry
			if err := rows.Scan(&last, &e.ID, &e.Room, &e.Number, &e.Points, &e.Result, &e.Surrendered); err != nil {
				archiveError(w, err)
				return
			}
			entries = append(entries, e)
		}
		if err := rows.Err(); err != nil {
			archiveError(w, err)
			return
		}
		archiveJSON(w, map[string]any{"items": entries, "next": next})
	})
	mux.HandleFunc("GET /api/rounds/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// Archive views contain cards but have no need for player identity IDs.
		const identities = ",'$.seats[0].uid','$.seats[1].uid','$.seats[2].uid','$.seats[3].uid'"
		var initial json.RawMessage
		var fromDeal bool
		err := db.QueryRowContext(r.Context(), "SELECT json_remove(initial_state"+identities+"),from_deal FROM rounds WHERE id=? AND "+settled, id).Scan((*[]byte)(&initial), &fromDeal)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			archiveError(w, err)
			return
		}
		rows, err := db.QueryContext(r.Context(), "SELECT seat,event,payload,json_remove(state_after"+identities+") FROM actions WHERE round_id=? ORDER BY version", id)
		if err != nil {
			archiveError(w, err)
			return
		}
		defer rows.Close()
		type step struct {
			Seat    int             `json:"seat"`
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
			State   json.RawMessage `json:"state"`
		}
		actions := []step{}
		for rows.Next() {
			var s step
			if err := rows.Scan(&s.Seat, &s.Event, (*[]byte)(&s.Payload), (*[]byte)(&s.State)); err != nil {
				archiveError(w, err)
				return
			}
			actions = append(actions, s)
		}
		if err := rows.Err(); err != nil {
			archiveError(w, err)
			return
		}
		archiveJSON(w, map[string]any{"initial": initial, "fromDeal": fromDeal, "actions": actions})
	})
}

func archiveError(w http.ResponseWriter, err error) {
	slog.Error("read archive", "error", err)
	http.Error(w, "暂时无法读取小局档案", http.StatusInternalServerError)
}
func archiveJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}
