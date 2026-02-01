package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

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

	uid    string
	roomID string
}

type HelloMsg struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func (c *Conn) sendHello() {
	msg := HelloMsg{
		Type: "hello",
		UID:  c.uid,
	}
	b, _ := json.Marshal(msg)
	c.send <- b
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
	default:
		// 发送队列满：直接丢弃或断开（这里先断开更容易发现问题）
		return websocket.ErrCloseSent
	}
	return nil
}

func (c *Conn) Close() error {
	close(c.send)
	return c.ws.Close()
}

func ServeWS(hub *Hub, router Router, w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	uid := r.URL.Query().Get("uid")
	if uid == "" {
		uid = "anon-" + time.Now().Format("150405.000")
	}
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	c := &Conn{
		ws:     wsConn,
		send:   make(chan []byte, 64),
		uid:    uid,
		roomID: roomID,
	}

	hub.register <- c
	router.OnConnect(c)

	go c.writeLoop()
	c.readLoop(router)

	router.OnDisconnect(c)
	hub.unregister <- c
	_ = wsConn.Close()
}

func (c *Conn) readLoop(router Router) {
	_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
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
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
