package engine

import (
	"log"
	"sync"
)

type Manager struct {
	clients    map[string]*Client
	clientsMu  sync.RWMutex
	Register   *Client
	Unregister *Client
}

func NewManager() *Manager {
	return &Manager {
		clients: make(map[string]*Client),
		Register:   *Client,
		Unregister: *Client

	}
}

func (m *Manager) Start() {
	for{
		select {
		case client := <-m.Register:
			m.clientsMu.Lock()
			m.clients[client.ID] = client
			m.clientsMu.Unlock()
			log.Printf("New Client Connected: %s", client.ID)
		
		case client := <-m.Unregister: 
			m.clientsMu.Lock()
			if _, ok := m.clients[client.ID]; ok {
				delete(m.clients, client.ID)
				close(client.Send())
			}
			m.clientsMu.Unlock()
			log.Printf("Client disconnected: %s", client.ID)
		}

	}
}