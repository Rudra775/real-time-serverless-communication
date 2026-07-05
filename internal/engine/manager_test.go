package engine

import (
	"context"
	"testing"
	"time"
)

func TestManager_RegisterUnregister(t *testing.T) {
	mgr := NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mgr.Start(ctx)

	client := &Client{
		ID:       "user_abc",
		Channels: map[string]bool{"general": true},
		Send:     make(chan []byte, 10),
	}

	// Register
	mgr.Register <- client

	// Let the manager process the registration
	time.Sleep(50 * time.Millisecond)

	mgr.clientsMu.RLock()
	c, ok := mgr.clients[client.ID]
	mgr.clientsMu.RUnlock()

	if !ok || c != client {
		t.Errorf("Client not registered correctly")
	}

	// Unregister
	mgr.Unregister <- client

	time.Sleep(50 * time.Millisecond)

	mgr.clientsMu.RLock()
	_, ok = mgr.clients[client.ID]
	mgr.clientsMu.RUnlock()

	if ok {
		t.Errorf("Client not unregistered correctly")
	}
}

func TestManager_ChannelRouting(t *testing.T) {
	mgr := NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mgr.Start(ctx)

	// Client 1 subscribed to roomA
	client1 := &Client{
		ID:       "client_1",
		Channels: map[string]bool{"roomA": true},
		Send:     make(chan []byte, 10),
	}

	// Client 2 subscribed to roomB
	client2 := &Client{
		ID:       "client_2",
		Channels: map[string]bool{"roomB": true},
		Send:     make(chan []byte, 10),
	}

	mgr.Register <- client1
	mgr.Register <- client2
	time.Sleep(50 * time.Millisecond)

	// Broadcast to roomA
	payloadA := []byte(`{"msg": "Hello roomA"}`)
	mgr.Broadcast <- ChannelMessage{
		Channel: "roomA",
		Payload: payloadA,
	}

	// Client 1 should receive it
	select {
	case msg := <-client1.Send:
		if string(msg) != string(payloadA) {
			t.Errorf("Client 1 expected %s, got %s", string(payloadA), string(msg))
		}
	case <-time.After(500 * time.Millisecond):
		t.Errorf("Client 1 did not receive message for roomA")
	}

	// Client 2 should NOT receive it
	select {
	case msg := <-client2.Send:
		t.Errorf("Client 2 received message for roomA but was subscribed to roomB: %s", string(msg))
	case <-time.After(100 * time.Millisecond):
		// Success: Client 2 did not receive it
	}
}
