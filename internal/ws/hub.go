package ws

type Hub struct {
	register   chan *Conn
	unregister chan *Conn

	byUID map[string]*Conn
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
		byUID:      make(map[string]*Conn),
	}
}

/*
用户不填 UID → 前端连接：/ws?room=room1
后端生成 anon-150405.000 → hello.uid=anon-... → seat/online 逻辑完全照旧

用户填 UID=alice → 前端连接：/ws?room=room1&uid=alice
后端采用 alice → hello.uid=alice
此后：入座、断线重连、snapshot 都以 alice 身份一致

两个客户端都用 alice（如果启用 Hub 冲突处理）
后连接者顶掉旧连接，旧连接收到 notice 并被 close
*/

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			// 同 UID 踢掉旧连接
			if old, ok := h.byUID[c.uid]; ok && old != c {
				_ = old.SendJSON(map[string]any{
					"type":    "notice",
					"message": "该UID在其他位置登录，你已被顶下线",
				})
				_ = old.Close()
			}
			h.byUID[c.uid] = c

		case c := <-h.unregister:
			if cur, ok := h.byUID[c.uid]; ok && cur == c {
				delete(h.byUID, c.uid)
			}
		}
	}
}
