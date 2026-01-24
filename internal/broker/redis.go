package broker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisBroker struct {
	Client *redis.Client
}

func NewRedisBroker(addr string) (*RedisBroker, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:        addr,
		MaxRetries:  3,
		DialTimeout: 5 * time.Second,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	log.Println("Redis connected successfully")
	return &RedisBroker{Client: rdb}, nil
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

	pipe := b.Client.Pipeline()
	pipe.RPush(ctx, key, msg)
	pipe.Expire(ctx, key, 24*time.Hour) //Message Expires before 24h

	_, err := pipe.Exec(ctx)

	return err
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

// GetAndClearInbox atomically retrieves and deletes messages
func (b *RedisBroker) ClearInbox(ctx context.Context, userID string) ([][]byte, error) {
	key := "inbox:" + userID

	// Use a Lua script for atomicity
	script := redis.NewScript(`
        local msgs = redis.call('LRANGE', KEYS[1], 0, -1)
        redis.call('DEL', KEYS[1])
        return msgs
    `)

	result, err := script.Run(ctx, b.Client, []string{key}).Result()
	if err != nil {
		return nil, err
	}

	// Convert to [][]byte
	stringSlice := result.([]interface{})
	msgs := make([][]byte, len(stringSlice))
	for i, v := range stringSlice {
		msgs[i] = []byte(v.(string))
	}

	return msgs, nil
}
