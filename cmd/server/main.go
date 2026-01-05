package main

import (
	"log"
	"net/http"

	"github.com/Rudra775/real-time-serverless-communication/internal/engine"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// 1. Initialize the Engine
	mgr := engine.NewManager()
	go mgr.Start() // Run the manager in a background goroutine

	// 2. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// 3. Define Routes
	r.Get("/connect", func(w http.ResponseWriter, r *http.Request) {
		// Simple ID generation for MVP (use UUID in production)
		clientID := r.URL.Query().Get("id")
		if clientID == "" {
			http.Error(w, "id param required", http.StatusBadRequest)
			return
		}

		client := &engine.Client{
			ID:   clientID,
			Send: make(chan []byte, 256), // Buffer up to 256 messages
		}

		// Register client with the manager
		mgr.Register <- client

		// Clean up on exit
		defer func() {
			mgr.Unregister <- client
		}()

		// Start the blocking SSE loop
		client.ServeSSE(w, r)
	})

	// 4. Start Server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
