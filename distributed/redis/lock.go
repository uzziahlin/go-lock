package redis

import (
	"context"
	"time"
)

type Lock struct {
	client     Client
	ExpireTime time.Duration
}

type LockOption func(lock *Lock)

func NewLock(opts ...LockOption) *Lock {

	client := NewDefaultClient(nil)

	res := &Lock{
		client: client,
	}

	for _, opt := range opts {
		opt(res)
	}

	return res
}

func (l Lock) Lock() {
	//TODO implement me
	panic("implement me")
}

func (l Lock) Unlock() {
	//TODO implement me
	panic("implement me")
}

func (l Lock) TryLock(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
