package engine

import "encoding/json"

// Message is the data structure sent to Redis and Clients
type Message struct {
	Type    string          `json:"type"`    // e.g., "chat", "notification"
	Payload json.RawMessage `json:"payload"` // Flexible JSON content
	RoomID  string          `json:"room_id"` // Target room (or user ID for now)
}
