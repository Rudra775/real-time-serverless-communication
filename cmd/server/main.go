package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Rudra775/real-time-serverless-communication/internal/broker"
	"github.com/Rudra775/real-time-serverless-communication/internal/engine"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	log.Printf("Connecting to Redis at %s...", redisAddr)
	redisBroker := broker.NewRedisBroker(redisAddr)

	// Initialize the Engine
	mgr := engine.NewManager()
	go mgr.Start() // Run the manager in a background goroutine

	// Start the "Subscriber" Loop (The Listener)
	// This goroutine listens to Redis and pushes messages into the Manager
	go func() {
		log.Println("Subscribing to Redis channel: 'general'...")
		ctx := context.Background()
		msgChan := redisBroker.Subscribe(ctx, "general") // Listening to "general" room for MVP

		for msg := range msgChan {
			mgr.Broadcast <- msg
		}
	}()

	// Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Define Routes
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

	// POST /send -> Publish to Redis
	r.Post("/send", func(w http.ResponseWriter, r *http.Request) {
		// Parse the body
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Prepare the message object
		msg := engine.Message{
			Type:    "broadcast",
			Payload: marshalPayload(payload), // Helper to convert map back to json.RawMessage
			RoomID:  "general",
		}

		// Convert to bytes
		data, _ := json.Marshal(msg)

		// Fire and Forget (Publish to Redis)
		// We use a short timeout context so we don't block forever if Redis is down
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := redisBroker.Publish(ctx, "general", data); err != nil {
			http.Error(w, "Failed to publish", http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Message sent"))
	})

	// 4. Start Server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

func marshalPayload(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
