package broker

import (
	"context"
	"os"
	"testing"
	"time"
)

func getTestBroker(t *testing.T) *RedisBroker {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	broker, err := NewRedisBroker(redisAddr)
	if err != nil {
		t.Skipf("Skipping integration test: Redis not available at %s", redisAddr)
		return nil
	}
	return broker
}

func TestRedisBroker_InboxOperations(t *testing.T) {
	broker := getTestBroker(t)
	if broker == nil {
		return
	}
	ctx := context.Background()
	userID := "test_user_123"

	// Ensure clean start
	broker.ClearInbox(ctx, userID)

	// 1. Save messages
	msg1 := []byte(`{"message": "Hello 1"}`)
	msg2 := []byte(`{"message": "Hello 2"}`)

	err := broker.SaveMessage(ctx, userID, msg1)
	if err != nil {
		t.Fatalf("Failed to save msg1: %v", err)
	}

	err = broker.SaveMessage(ctx, userID, msg2)
	if err != nil {
		t.Fatalf("Failed to save msg2: %v", err)
	}

	// 2. Get pending
	pending, err := broker.GetPendingMessages(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get pending messages: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending messages, got %d", len(pending))
	}

	if string(pending[0]) != string(msg1) || string(pending[1]) != string(msg2) {
		t.Errorf("Inbox message content mismatch")
	}

	// 3. Clear inbox
	cleared, err := broker.ClearInbox(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to clear inbox: %v", err)
	}

	if len(cleared) != 2 {
		t.Errorf("Expected to clear 2 messages, cleared %d", len(cleared))
	}

	// Verify empty now
	empty, err := broker.GetPendingMessages(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get empty pending messages: %v", err)
	}

	if len(empty) != 0 {
		t.Errorf("Expected empty inbox, got %d messages", len(empty))
	}
}

func TestRedisBroker_PubSubPattern(t *testing.T) {
	broker := getTestBroker(t)
	if broker == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pattern := "realtime:channel:test_room_*"
	channel := "realtime:channel:test_room_alpha"
	payload := []byte(`{"event": "test"}`)

	msgChan, cleanup, err := broker.SubscribePattern(ctx, pattern)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer cleanup()

	// Wait briefly for subscription to register
	time.Sleep(100 * time.Millisecond)

	// Publish message
	err = broker.Publish(ctx, channel, payload)
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Read message
	select {
	case msg, ok := <-msgChan:
		if !ok {
			t.Fatalf("Channel closed prematurely")
		}
		if msg.Channel != channel {
			t.Errorf("Expected channel %s, got %s", channel, msg.Channel)
		}
		if string(msg.Payload) != string(payload) {
			t.Errorf("Expected payload %s, got %s", string(payload), string(msg.Payload))
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Timed out waiting for pub/sub message")
	}
}
