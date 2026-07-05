package engine

import (
	"context" // Add this
	"log"
	"sync"
)

type ChannelMessage struct {
	Channel string
	Payload []byte
}

type Manager struct {
	clients    map[string]*Client
	clientsMu  sync.RWMutex
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan ChannelMessage
}

func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan ChannelMessage),
	}
}

// Update Start to accept a Context for graceful shutdown
func (m *Manager) Start(ctx context.Context) {
	defer func() {
		// Cleanup when manager stops
		m.clientsMu.Lock()
		for _, client := range m.clients {
			close(client.Send)
		}
		m.clientsMu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			// 1. Graceful Shutdown Signal
			log.Println("Manager stopping...")
			return

		case client := <-m.Register:
			m.clientsMu.Lock()
			m.clients[client.ID] = client
			m.clientsMu.Unlock()
			log.Printf("Client registered: %s", client.ID)

		case client := <-m.Unregister:
			m.clientsMu.Lock()
			if _, ok := m.clients[client.ID]; ok {
				delete(m.clients, client.ID)
				close(client.Send)
			}
			m.clientsMu.Unlock()
			log.Printf("Client unregistered: %s", client.ID)

		case msg := <-m.Broadcast:
			m.clientsMu.Lock() // Use write lock for broadcasts
			for id, client := range m.clients {
				// Only send if the client is subscribed to this channel
				if client.IsSubscribed(msg.Channel) {
					select {
					case client.Send <- msg.Payload:
						// Success
					default:
						// Buffer full - delete immediately
						delete(m.clients, id)
						close(client.Send)
						log.Printf("Client %s buffer full, dropped", id)
					}
				}
			}
			m.clientsMu.Unlock()
		}
	}
}

