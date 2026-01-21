package engine

import (
	"context" // Add this
	"log"
	"sync"
)

type Manager struct {
	clients    map[string]*Client
	clientsMu  sync.RWMutex
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan []byte
}

func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte),
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
			m.clientsMu.RLock()
			for id, client := range m.clients {
				select {
				case client.Send <- msg:
					// Message sent successfully
				default:
					// 2. CRITICAL FIX: Buffer is full.
					// Just close the channel. Do NOT delete from map here.
					// Let the Unregister channel handle the cleanup safely.
					close(client.Send)
					log.Printf("Client %s buffer full, dropping connection.", id)
				}
			}
			m.clientsMu.RUnlock()
		}
	}
}
