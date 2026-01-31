package ws

type Hub struct {
	register   chan *Conn
	unregister chan *Conn
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case <-h.register:
		case <-h.unregister:
		}
	}
}
