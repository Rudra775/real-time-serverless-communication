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
func (b *RedisBroker) Subscribe(ctx context.Context, channel string) <-chan []byte {
	pubsub := b.Client.Subscribe(ctx, channel)
	msgChan := make(chan []byte)

	// Start a goroutine to pump messages from Redis to our channel
	go func() {
		ch := pubsub.Channel()
		for msg := range ch {
			msgChan <- []byte(msg.Payload)
		}
		close(msgChan)
	}()

	return msgChan
}
