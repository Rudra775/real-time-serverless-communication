package engine

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func (c *Client) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// 1. Set Headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Nginx buffering fix (Crucial for your load balancer!)
	w.Header().Set("X-Accel-Buffering", "no")

	// 2. Ensure streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// 3. Send initial connection confirmation
	fmt.Fprintf(w, "data: {\"type\":\"connected\", \"user_id\":\"%s\"}\n\n", c.ID)
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
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()

		// Case B: Heartbeat Tick
		case <-ticker.C:
			// Send a comment line (starts with :) to keep connection alive
			// Browsers ignore lines starting with colon
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		// Case C: Client Disconnected (Browser closed tab)
		case <-r.Context().Done():
			log.Printf("Client disconnected: %s", c.ID)
			return
		}
	}
}
