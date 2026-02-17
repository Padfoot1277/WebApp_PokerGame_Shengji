package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
	"upgrade-lan/internal/transport"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // LAN demo
}

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Router ws 收到消息后交给上层（room.Manager）处理
type Router interface {
	OnConnect(c transport.Client)
	OnDisconnect(c transport.Client)
	OnMessage(c transport.Client, typ string, payload json.RawMessage)
}

type Conn struct {
	ws   *websocket.Conn
	send chan []byte
	done chan struct{}

	closeOnce sync.Once

	uid    string
	roomID string
}

type HelloMsg struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func (c *Conn) UID() string    { return c.uid }
func (c *Conn) RoomID() string { return c.roomID }

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
		return websocket.ErrCloseSent
	}
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
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	uid := normalizeAnyUID(r.URL.Query().Get("uid"))
	if uid == "" {
		uid = time.Now().Format("150405")
	}

	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	c := &Conn{
		ws:     wsConn,
		send:   make(chan []byte, 64),
		done:   make(chan struct{}),
		uid:    uid,
		roomID: roomID,
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
	_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

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

		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			_ = c.SendJSON(map[string]any{
				"type":    "error",
				"code":    "bad_json",
				"message": "invalid json envelope",
			})
			continue
		}

		router.OnMessage(c, env.Type, env.Payload)
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
