package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

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
	// Pass a background context, or a cancelable context if you want to support shutdown
	go mgr.Start(context.Background())

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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

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

		log.Printf("Checking inbox for %s...", clientID)
		missedMsgs, _ := redisBroker.GetPendingMessages(r.Context(), clientID)

		// Push missed messages to the client immediately
		for _, msgBytes := range missedMsgs {
			client.Send <- msgBytes
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
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// 1. Check if this is a Direct Message
		targetUser, _ := payload["to_user"].(string)

		msg := engine.Message{
			Type:    "broadcast",
			Payload: marshalPayload(payload),
			RoomID:  "general",
		}
		data, _ := json.Marshal(msg)

		ctx := context.Background()

		// 2. RELIABILITY FIX: If it's for a specific user, save to their inbox!
		if targetUser != "" {
			log.Printf("Saving message to inbox for %s", targetUser)
			// Save to Redis List "inbox:targetUser"
			redisBroker.SaveMessage(ctx, targetUser, data)
		}

		// 3. Publish as usual (Real-time delivery)
		redisBroker.Publish(ctx, "general", data)

		w.Write([]byte("Message sent"))
	})

	r.Post("/ack", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		userId := payload["user_id"]
		if userId == "" {
			http.Error(w, "Missing user_id", http.StatusBadRequest)
			return
		}

		redisBroker.ClearInbox(r.Context(), userId)
		w.Write([]byte("Acknowledged"))
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
