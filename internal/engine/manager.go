package engine

import (
	"log"
	"sync"
)

// Manager keeps track of all active user sessions
type Manager struct {
	clients    map[string]*Client // Map of SessionID -> Client
	clientsMu  sync.RWMutex       // Protects the map from concurrent access
	Register   chan *Client       // Channel to add new clients safely
	Unregister chan *Client       // Channel to remove disconnected clients

	Broadcast chan []byte
}

// NewManager creates the central hub
func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte),
	}
}

// Start begins the event loop (run this in a goroutine)
func (m *Manager) Start() {
	for {
		select {
		case client := <-m.Register:
			m.clientsMu.Lock()
			m.clients[client.ID] = client
			m.clientsMu.Unlock()
			log.Printf("New client connected: %s", client.ID)

		case client := <-m.Unregister:
			m.clientsMu.Lock()
			if _, ok := m.clients[client.ID]; ok {
				delete(m.clients, client.ID)
				close(client.Send) // Close the channel to stop the goroutine
			}
			m.clientsMu.Unlock()
			log.Printf("Client disconnected: %s", client.ID)

		case msg := <-m.Broadcast:
			// When we receive a message, send it to ALL connected clients
			// (For MVP, we broadcast to everyone. Later we will filter by RoomID)
			m.clientsMu.RLock()
			for _, client := range m.clients {
				select {
				case client.Send <- msg:
				default:
					close(client.Send)
					delete(m.clients, client.ID)
				}
			}
			m.clientsMu.RUnlock()
		}

	}
}
