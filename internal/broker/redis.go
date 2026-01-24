package broker

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisBroker struct {
	Client *redis.Client
}

func NewRedisBroker(addr string) *RedisBroker {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisBroker{Client: rdb}
}

// Publish sends a message to a specific channel (room)
func (b *RedisBroker) Publish(ctx context.Context, channel string, msg []byte) error {
	return b.Client.Publish(ctx, channel, msg).Err()
}

// Subscribe listens to a channel and passes messages to a go channel
func (b *RedisBroker) Subscribe(ctx context.Context, channel string) (<-chan []byte, func(), error) {
	pubsub := b.Client.Subscribe(ctx, channel)

	//Check if subscription succeeded
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, nil, err
	}

	msgChan := make(chan []byte)

	// Cleanup function
	cleanup := func() {
		pubsub.Close()
	}

	// Start a goroutine to pump messages from Redis to our channel
	go func() {
		defer close(msgChan)
		ch := pubsub.Channel()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				select {
				case msgChan <- []byte(msg.Payload):
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return msgChan, cleanup, nil
}

// SaveMessage stores a message in a user's personal inbox (List)
func (b *RedisBroker) SaveMessage(ctx context.Context, userID string, msg []byte) error {
	key := "inbox:" + userID
	return b.Client.RPush(ctx, key, msg).Err()
}

// GetPendingMessages retrieves all messages currently in the inbox
func (b *RedisBroker) GetPendingMessages(ctx context.Context, userID string) ([][]byte, error) {
	key := "inbox:" + userID
	// LRANGE 0 -1 gets EVERYTHING in the list
	result, err := b.Client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	// Convert string slice to byte slice
	msgs := make([][]byte, len(result))
	for i, s := range result {
		msgs[i] = []byte(s)
	}
	return msgs, nil
}

func (b *RedisBroker) ClearInbox(ctx context.Context, userId string) error {
	key := "inbox:" + userId

	return b.Client.Del(ctx, key).Err()
}
