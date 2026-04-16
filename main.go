package main

import (
	"log"
	"net/http"

	"capstone_sse/internal/config"
	"capstone_sse/internal/rabbitmq"
	"capstone_sse/internal/sse"
)

func main() {
	cfg := config.Load()
	hub := sse.NewHub()

	go rabbitmq.StartConsumer(hub, cfg)

	http.HandleFunc("/sse/connect", sse.NewConnectHandler(hub))

	addr := ":" + cfg.ServerPort
	log.Printf("SSE gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
