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
	Subscribe(ctx context.Context, channels ...string) (chan any, error)
}

type Result struct {
	Val any
	Err error
}

type DefaultClient struct {
	client *redis.Client
}

func (d DefaultClient) Eval(ctx context.Context, script string, keys []string, val ...any) *Result {

	res, err := d.client.Eval(ctx, script, keys, val).Result()

	return &Result{
		Val: res,
		Err: err,
	}
}

func (d DefaultClient) Subscribe(ctx context.Context, channels ...string) (chan any, error) {
	res := make(chan any)

	sub := d.client.Subscribe(ctx, channels...)

	go func() {
		for {
			receive, err := sub.Receive(ctx)
			if err != nil {
				res <- err
				return
			}
			switch v := receive.(type) {
			case *redis.Subscription:
				res <- &Subscription{
					Kind:    v.Kind,
					Channel: v.Channel,
					Count:   v.Count,
				}
			case *redis.Message:
				res <- &Message{
					Channel: v.Channel,
					Pattern: v.Pattern,
					Data:    []byte(v.Payload),
				}
				for _, p := range v.PayloadSlice {
					res <- &Message{
						Channel: v.Channel,
						Pattern: v.Pattern,
						Data:    []byte(p),
					}
				}
			case *Pong:
				res <- &Pong{
					Data: v.Data,
				}
			case error:
				res <- v
			}
		}
	}()

	return res, nil
}

func NewDefaultClient(opts *redis.Options) *DefaultClient {

	return &DefaultClient{
		client: redis.NewClient(opts),
	}

}
