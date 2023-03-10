package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
)

type Client interface {
	Eval(ctx context.Context, script string, keys []string, val ...any) *Result
	Subscribe(channels ...any) error
}

type Result struct {
	Val any
	Err error
}

type DefaultClient struct {
	client redis.Cmdable
}

func (d DefaultClient) Eval(ctx context.Context, script string, keys []string, val ...any) *Result {
	//TODO implement me
	panic("implement me")
}

func (d DefaultClient) Subscribe(channels ...any) error {
	//TODO implement me
	panic("implement me")
}

func NewDefaultClient(client redis.Cmdable) *DefaultClient {

	if client == nil {
		client = &redis.Client{}
	}

	return &DefaultClient{
		client: client,
	}
}
