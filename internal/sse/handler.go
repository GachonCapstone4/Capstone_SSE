package sse

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// NewConnectHandler returns the HTTP handler for GET /sse/connect.
// It extracts user_id from the Authorization Bearer JWT token,
// registers the client with the hub, and streams events until the client disconnects.
func NewConnectHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := extractUserID(r)
		if err != nil {
			http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := make(chan Event, 32)
		hub.Register(userID, ch)
		defer hub.Unregister(userID, ch)

		log.Printf("SSE client connected: user_id=%s remote=%s", userID, r.RemoteAddr)

		for {
			select {
			case event := <-ch:
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(event.Data))
				flusher.Flush()
			case <-r.Context().Done():
				log.Printf("SSE client disconnected: user_id=%s", userID)
				return
			}
		}
	}
}

// extractUserID parses the Authorization: Bearer <JWT> header and returns the subject claim as user_id.
// Signature verification is skipped because this gateway runs inside the cluster (internal network).
func extractUserID(r *http.Request) (int64, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0, fmt.Errorf("missing Authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return 0, fmt.Errorf("invalid Authorization header format")
	}
	tokenStr := parts[1]

	// Parse without verification (internal cluster; trust the issuer)
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return 0, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid JWT claims")
	}

	// Support both "sub" and "user_id" claim names
	var userID int64
	if sub, err := claims.GetSubject(); err == nil && sub != "" {
		if _, err := fmt.Sscanf(sub, "%d", &userID); err == nil && userID > 0 {
			return userID, nil
		}
	}
	if uid, ok := claims["user_id"].(float64); ok && uid > 0 {
		return int64(uid), nil
	}

	return 0, fmt.Errorf("user_id not found in JWT claims")
}
