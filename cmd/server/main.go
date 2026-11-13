package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"upgrade-lan/internal/bot"
	"upgrade-lan/internal/room"
	"upgrade-lan/internal/ws"
)

func main() {
	hub := ws.NewHub()
	go hub.Run()

	var model *bot.LearnModel
	mode := env("BOT_MODEL_MODE", "search")
	if path := os.Getenv("BOT_MODEL_PATH"); path != "" {
		if mode != "search" && mode != "hybrid" {
			log.Fatalf("unknown BOT_MODEL_MODE %q, expected search or hybrid", mode)
		}
		var err error
		model, err = bot.LoadLearnedPolicy(path)
		if err != nil {
			log.Printf("bot checkpoint %q unavailable, using built-in policy: %v", path, err)
		} else {
			log.Printf("loaded bot checkpoint %q, mode %s", path, mode)
		}
	}
	if model == nil {
		policy := bot.RoomRuleSearch()
		log.Printf("room bots using rule search: max_hand=%d candidates=%d exact_tail=%d", policy.MaxHand, policy.Candidates, policy.ExactTail)
	}
	rm := room.NewManagerWithBotModel(model, mode == "hybrid")
	cleanup := func() {
		if err := rm.CleanupExpired(time.Now()); err != nil {
			log.Println("expired room cleanup:", err)
		}
	}
	cleanup()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()

	room.RegisterArchive(http.DefaultServeMux)
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
