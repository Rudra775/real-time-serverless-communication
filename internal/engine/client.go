package engine

import "net/http"

type Client struct {
	ID   string
	Send chan []byte // Channel for messages waiting to be sent to this user
}

// ServeSSE handles the infinite loop of writing data to the browser
func (c *Client) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// 1. Set Headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// 2. Listen for messages or context cancellation
	for {
		select {
		case msg, open := <-c.Send:
			if !open {
				return // Channel closed by Manager
			}
			// Write data in SSE format: "data: <payload>\n\n"
			w.Write([]byte("data: "))
			w.Write(msg)
			w.Write([]byte("\n\n"))
			flusher.Flush() // Send immediately

		case <-r.Context().Done():
			// Browser closed connection
			return
		}
	}
}
