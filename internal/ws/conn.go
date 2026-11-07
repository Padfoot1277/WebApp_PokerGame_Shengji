package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
	"upgrade-lan/internal/transport"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if u, err := url.Parse(origin); err == nil && u.Host == r.Host {
			return true
		}
		for _, allowed := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
			if origin == strings.TrimSpace(allowed) {
				return true
			}
		}
		return false
	},
}

type Envelope struct {
	Type    string          `json:"type"`
	ReqID   string          `json:"reqId"`
	Payload json.RawMessage `json:"payload"`
}

// Router ws 收到消息后交给上层（room.Manager）处理
type Router interface {
	OnConnect(c transport.Client)
	OnDisconnect(c transport.Client)
	OnMessage(c transport.Client, typ string, payload json.RawMessage, reqID string)
}

type Conn struct {
	ws   *websocket.Conn
	send chan []byte
	done chan struct{}

	closeOnce  sync.Once
	identityMu sync.RWMutex

	name                  string
	token                 string
	uid                   string
	roomID                string
	recordEnabled         bool
	allowNicknameRecovery bool
}

type HelloMsg struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func (c *Conn) Name() string        { return c.name }
func (c *Conn) Token() string       { c.identityMu.RLock(); defer c.identityMu.RUnlock(); return c.token }
func (c *Conn) UID() string         { c.identityMu.RLock(); defer c.identityMu.RUnlock(); return c.uid }
func (c *Conn) RoomID() string      { return c.roomID }
func (c *Conn) RecordEnabled() bool { return c.recordEnabled }

// Done lets the room reject a connection replaced before its join was processed.
func (c *Conn) Done() <-chan struct{} { return c.done }

func (c *Conn) SendJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	select {
	case c.send <- b:
		return nil
	case <-c.done:
		// 已关闭
		return websocket.ErrCloseSent
	default:
		// 发送队列满，认为连接异常
		_ = c.Close()
		return websocket.ErrCloseSent
	}
}

func (c *Conn) Replace() {
	_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "session replaced"), time.Now().Add(time.Second))
	_ = c.Close()
}
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)      // 通知所有 goroutine 退出
		err = c.ws.Close() // 关闭底层 websocket
	})
	return err
}

func ServeWS(hub *Hub, router Router, w http.ResponseWriter, r *http.Request) {
	id, token, err := session(r.URL.Query().Get("token"), r.URL.Query().Get("name"))
	if err != nil {
		socket, e := upgrader.Upgrade(w, r, nil)
		if e == nil {
			_ = socket.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4003, "session expired"), time.Now().Add(time.Second))
			_ = socket.Close()
		}
		return
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	uid := id.UID

	roomID := r.URL.Query().Get("room")
	if len([]rune(roomID)) > 32 {
		_ = wsConn.Close()
		return
	}
	if roomID == "" {
		roomID = "default"
	}

	c := &Conn{
		ws:   wsConn,
		send: make(chan []byte, 64),
		done: make(chan struct{}),
		uid:  uid,
		name: id.Name, token: token,
		roomID:                roomID,
		recordEnabled:         r.URL.Query().Get("record") == "1",
		allowNicknameRecovery: r.URL.Query().Get("recover") == "1",
	}

	hub.register <- c
	router.OnConnect(c)

	go c.writeLoop()
	c.readLoop(router)

	// readLoop 退出说明连接断开
	router.OnDisconnect(c)
	hub.unregister <- c
	_ = c.Close()
}

func (c *Conn) readLoop(router Router) {
	c.ws.SetReadLimit(16 * 1024)
	_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	windowStart := time.Now()
	requests := 0
	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		// Count malformed envelopes too; JSON decoding must not bypass the budget.
		if time.Since(windowStart) > time.Second {
			windowStart = time.Now()
			requests = 0
		}
		requests++
		if requests > 30 {
			_ = c.Close()
			return
		}

		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			_ = c.SendJSON(map[string]any{
				"type":    "error",
				"code":    "bad_json",
				"message": "invalid json envelope",
			})
			continue
		}

		if len(env.ReqID) > 80 {
			_ = c.Close()
			return
		}
		if env.ReqID == "" {
			_ = c.SendJSON(map[string]any{"type": "error", "code": "MISSING_REQ_ID", "message": "请求缺少编号"})
			continue
		}
		router.OnMessage(c, env.Type, env.Payload, env.ReqID)
	}
}

func (c *Conn) writeLoop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				_ = c.Close()
				return
			}

		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = c.Close()
				return
			}

		case <-c.done:
			return
		}
	}
}

func normalizeAnyUID(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// 移除控制字符
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)

	// 限制长度
	const maxRunes = 24
	rs := []rune(s)
	if len(rs) > maxRunes {
		s = string(rs[:maxRunes])
	}
	return s
}

func (c *Conn) RecoverSeat(uid string) bool {
	if !c.allowNicknameRecovery {
		return false
	}
	_, token, err := signIdentity(identity{UID: uid, Name: c.name, Expires: time.Now().Add(7 * 24 * time.Hour).Unix()})
	if err != nil {
		return false
	}
	c.identityMu.Lock()
	c.uid, c.token = uid, token
	c.identityMu.Unlock()
	return true
}
