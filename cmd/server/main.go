package main

import (
	"log"
	"net/http"
	"os"
	"time"

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

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"shengji","status":"ok"}`))
	})
	http.Handle("/", http.FileServer(http.Dir(env("WEB_DIR", "./frontend/dist"))))

	addr := env("ADDR", ":8080")
	log.Println("server listening on", addr)
	log.Fatal((&http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}).ListenAndServe())
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
