package main

import (
	"log"
	"net/http"

	"capstone_sse/internal/config"
	"capstone_sse/internal/rabbitmq"
	"capstone_sse/internal/sse"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://mymaily.vercel.app")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Cache-Control")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	hub := sse.NewHub()
	go rabbitmq.StartConsumer(hub, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse/connect", sse.NewConnectHandler(hub))

	addr := ":" + cfg.ServerPort
	log.Printf("SSE gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, corsMiddleware(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
