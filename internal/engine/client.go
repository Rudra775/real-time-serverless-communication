package engine

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// Client represents a single connected user
type Client struct {
	ID       string          // Unique ID (e.g., user_123)
	Channels map[string]bool // Set of channels this client is subscribed to
	Send     chan []byte     // Buffered channel for outgoing messages
}

// IsSubscribed checks if the client is subscribed to the given channel
func (c *Client) IsSubscribed(channel string) bool {
	if c.Channels == nil {
		return false
	}
	return c.Channels[channel]
}


// ServeSSE establishes a Server-Sent Events stream for real-time message delivery.
// This is designed for use behind a reverse proxy (Nginx/Cloudflare) that handles TLS.
//
// Key Features:
// - Nginx-compatible (X-Accel-Buffering header)
// - 15-second heartbeat prevents proxy timeouts
// - Graceful shutdown on channel close
// - Automatic cleanup on client disconnect
//
// For production use: Run behind a load balancer with TLS termination.

func (c *Client) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// 1. Set Headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Nginx buffering fix (Crucial for load balancer!)
	w.Header().Set("X-Accel-Buffering", "no")

	// 2. Ensure streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 3. Send initial connection confirmation
	if _, err := fmt.Fprintf(w, "data: {\"type\":\"connected\", \"user_id\":\"%s\"}\n\n", c.ID); err != nil {
		log.Printf("Failed to send connected message to %s: %v", c.ID, err)
		return
	}
	flusher.Flush()
	log.Printf("Client connected: %s", c.ID)

	// 4. Create a Ticker for Heartbeats (Keep-Alive)
	// Send a 'ping' every 15 seconds so Nginx doesn't kill the connection
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// 5. Main Loop
	for {
		select {
		// Case A: Broadcast Message received
		case msg, open := <-c.Send:
			if !open {
				// Channel closed by the manager (Graceful shutdown)
				fmt.Fprintf(w, "data: {\"type\":\"close\"}\n\n")
				flusher.Flush()
				return
			}
			// Write the actual message
			// Note: We assume 'msg' is already JSON bytes
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				log.Printf("Failed to send message to %s: %v", c.ID, err)
				return
			}
			flusher.Flush()

		// Case B: Heartbeat Tick
		case <-ticker.C:
			// Send a comment line (starts with :) to keep connection alive
			// Browsers ignore lines starting with colon
			// CHANGE: Error handling on keepalive
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				log.Printf("Keepalive failed for %s: %v", c.ID, err)
				return
			}
			flusher.Flush()

		// Case C: Client Disconnected (Browser closed tab)
		case <-r.Context().Done():
			log.Printf("Client disconnected: %s", c.ID)
			return
		}
	}
}
