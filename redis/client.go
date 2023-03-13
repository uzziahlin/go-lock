package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
)

type Subscription struct {
	// Kind is "subscribe", "unsubscribe", "psubscribe" or "punsubscribe"
	Kind string

	// The channel that was changed.
	Channel string

	// The current number of subscriptions for connection.
	Count int
}

type Message struct {
	// The originating channel.
	Channel string

	// The matched pattern, if any
	Pattern string

	// The message data.
	Data []byte
}

type Pong struct {
	Data string
}

type Client interface {
	Eval(ctx context.Context, script string, keys []string, val ...any) *Result
	Subscribe(ctx context.Context, channels ...any) (chan any, error)
}

type Result struct {
	Val any
	Err error
}

type DefaultClient struct {
	client *redis.Client
}

func (d DefaultClient) Eval(ctx context.Context, script string, keys []string, val ...any) *Result {
	//TODO implement me
	panic("implement me")
}

func (d DefaultClient) Subscribe(ctx context.Context, channels ...any) (chan any, error) {
	//TODO implement me
	panic("implement me")
}

func NewDefaultClient() *DefaultClient {

	return &DefaultClient{
		client: &redis.Client{},
	}
}
