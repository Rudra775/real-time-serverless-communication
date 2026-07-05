package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Rudra775/real-time-serverless-communication/internal/broker"
	"github.com/Rudra775/real-time-serverless-communication/internal/engine"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	log.Printf("Connecting to Redis at %s...", redisAddr)
	redisBroker, err := broker.NewRedisBroker(redisAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the Engine
	mgr := engine.NewManager()
	go mgr.Start(context.Background())

	// Start the "Subscriber" Loop (The Listener)
	// This goroutine listens to Redis channel pattern 'realtime:channel:*'
	go func() {
		log.Println("Subscribing to Redis channel pattern: 'realtime:channel:*'...")
		ctx := context.Background()
		msgChan, cleanup, err := redisBroker.SubscribePattern(ctx, "realtime:channel:*")
		if err != nil {
			log.Fatal(err)
		}
		defer cleanup()

		for msg := range msgChan {
			// Extract channel name from format: "realtime:channel:{channelName}"
			parts := strings.SplitN(msg.Channel, "realtime:channel:", 2)
			if len(parts) < 2 {
				continue
			}
			channelName := parts[1]

			mgr.Broadcast <- engine.ChannelMessage{
				Channel: channelName,
				Payload: []byte(msg.Payload),
			}
		}
	}()

	// Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

	r.Get("/connect", func(w http.ResponseWriter, r *http.Request) {
		var clientID string
		var channelsList []string

		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret != "" {
			// JWT Mode
			tokenStr := r.URL.Query().Get("token")
			if tokenStr == "" {
				http.Error(w, "token query param required in JWT mode", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid claims", http.StatusUnauthorized)
				return
			}

			// Extract User ID
			if sub, ok := claims["sub"].(string); ok {
				clientID = sub
			} else if uid, ok := claims["user_id"].(string); ok {
				clientID = uid
			} else {
				http.Error(w, "token missing subject/user_id", http.StatusUnauthorized)
				return
			}

			// Extract Channels
			if chans, ok := claims["channels"].([]interface{}); ok {
				for _, ch := range chans {
					if chStr, ok := ch.(string); ok {
						channelsList = append(channelsList, chStr)
					}
				}
			}
		} else {
			// Dev Mode (no JWT)
			clientID = r.URL.Query().Get("id")
			if clientID == "" {
				http.Error(w, "id param required in dev mode", http.StatusBadRequest)
				return
			}

			chansQuery := r.URL.Query().Get("channels")
			if chansQuery != "" {
				for _, ch := range strings.Split(chansQuery, ",") {
					chTrimmed := strings.TrimSpace(ch)
					if chTrimmed != "" {
						channelsList = append(channelsList, chTrimmed)
					}
				}
			}
		}

		// Always default to including the "general" channel
		hasGeneral := false
		for _, ch := range channelsList {
			if ch == "general" {
				hasGeneral = true
				break
			}
		}
		if !hasGeneral {
			channelsList = append(channelsList, "general")
		}

		// Build client channel set
		channelsMap := make(map[string]bool)
		for _, ch := range channelsList {
			channelsMap[ch] = true
		}

		client := &engine.Client{
			ID:       clientID,
			Channels: channelsMap,
			Send:     make(chan []byte, 256), // Buffer up to 256 messages
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

	// /ack handles client acknowledgments. Accessible publicly, but securely verified.
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

		// Allow bypass if a valid X-API-Key is provided (backend bypass)
		apiKey := os.Getenv("REALTIME_API_KEY")
		isAuthorized := false
		if apiKey != "" && (r.Header.Get("X-API-Key") == apiKey || r.URL.Query().Get("api_key") == apiKey) {
			isAuthorized = true
		}

		// Otherwise, enforce JWT validation if JWT_SECRET is configured
		jwtSecret := os.Getenv("JWT_SECRET")
		if !isAuthorized && jwtSecret != "" {
			tokenStr := payload["token"]
			if tokenStr == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenStr == "" {
				http.Error(w, "Unauthorized: token required in JWT mode", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
				return
			}

			var tokenUser string
			if sub, ok := claims["sub"].(string); ok {
				tokenUser = sub
			} else if uid, ok := claims["user_id"].(string); ok {
				tokenUser = uid
			}

			if tokenUser != userId {
				http.Error(w, "Unauthorized: token user mismatch", http.StatusUnauthorized)
				return
			}
		}

		redisBroker.ClearInbox(r.Context(), userId)
		w.Write([]byte("Acknowledged"))
	})

	// Secure publishing routes with API Key middleware (internal server only)
	r.Group(func(r chi.Router) {
		r.Use(apiKeyAuth)

		// POST /send -> Publish to Redis
		r.Post("/send", func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Target channel
			channelName, _ := payload["channel"].(string)
			if channelName == "" {
				channelName = "general" // default channel
			}

			// Check if this should be saved in specific users' inboxes
			var targetUsers []string
			if toUser, ok := payload["to_user"].(string); ok && toUser != "" {
				targetUsers = append(targetUsers, toUser)
			}
			if toUsers, ok := payload["to_users"].([]interface{}); ok {
				for _, u := range toUsers {
					if uStr, ok := u.(string); ok && uStr != "" {
						targetUsers = append(targetUsers, uStr)
					}
				}
			}

			msg := engine.Message{
				Type:    "broadcast",
				Payload: marshalPayload(payload),
				RoomID:  channelName,
			}
			data, _ := json.Marshal(msg)

			ctx := context.Background()

			// Save to targets' inboxes
			for _, targetUser := range targetUsers {
				log.Printf("Saving message to inbox for %s", targetUser)
				redisBroker.SaveMessage(ctx, targetUser, data)
			}

			// Publish to the specific channel
			redisChannel := "realtime:channel:" + channelName
			redisBroker.Publish(ctx, redisChannel, data)

			w.Write([]byte("Message sent"))
		})
	})

	// Start Server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// Middleware to authorize server-to-server requests via API Key
func apiKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := os.Getenv("REALTIME_API_KEY")
		if apiKey == "" {
			// No API key set: disable check for development convenience
			next.ServeHTTP(w, r)
			return
		}

		clientKey := r.Header.Get("X-API-Key")
		if clientKey == "" {
			clientKey = r.URL.Query().Get("api_key")
		}

		if clientKey != apiKey {
			http.Error(w, "Unauthorized: Invalid or missing X-API-Key header", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func marshalPayload(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
