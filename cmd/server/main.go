package main

import (
	"log"
	"net/http"

	"upgrade-lan/internal/room"
	"upgrade-lan/internal/ws"
)

func main() {
	hub := ws.NewHub()
	go hub.Run()

	rm := room.NewManager() // room 不再需要 hub/ws

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// ws 只依赖一个 Router 接口（rm 实现它）
		ws.ServeWS(hub, rm, w, r)
	})

	http.Handle("/", http.FileServer(http.Dir("./web")))

	addr := ":8080"
	log.Println("server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
