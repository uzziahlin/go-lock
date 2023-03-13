package redis

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

//go:embed scripts/redis_lock.lua
var lockScript string

//go:embed scripts/redis_expire.lua
var expireScript string

//go:embed scripts/redis_unlock.lua
var unlockScript string

const (
	LockChannelPrefix = "Unlock:"
	DefaultExpireTime = 30 * time.Second
)

type Lock struct {
	client Client
	// 锁的唯一标识，防止误删锁
	Id string
	// 锁的名字
	Name       string
	ExpireTime time.Duration
	// 表示是否开启了看门狗机制
	hasDog uint32

	doneC chan struct{}
}

type LockOption func(lock *Lock)

func NewLock(opts ...LockOption) *Lock {

	client := NewDefaultClient()

	res := &Lock{
		client: client,
	}

	for _, opt := range opts {
		opt(res)
	}

	return res
}

func (l *Lock) Lock(ctx context.Context) error {

	if ctx.Err() != nil {
		return ctx.Err()
	}

	expireTime := l.ExpireTime.Milliseconds()

	if expireTime == 0 {
		expireTime = 30 * time.Second.Milliseconds()
	}

	for {
		res := l.client.Eval(ctx, lockScript, []string{l.Name}, l.Id, expireTime)

		if res.Err != nil {
			return res.Err
		}

		// 如果val == nil 说明已经拿到锁了
		if res.Val == nil {

			// 如果用户没有设置过期时间则开启看门狗机制
			if l.ExpireTime == 0 && atomic.LoadUint32(&l.hasDog) == 0 {
				if atomic.CompareAndSwapUint32(&l.hasDog, 0, 1) {
					l.startWatchDog(ctx, DefaultExpireTime)
				}
			}

			return nil
		}

		sub := l.Subscribe(ctx)

		select {
		case v := <-sub:
			if v.Err != nil {
				return v.Err
			}
		case <-time.After(time.Duration(res.Val.(int64)) * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

type Sub struct {
	Err error
}

func (l *Lock) Subscribe(ctx context.Context) <-chan Sub {
	c := make(chan Sub)
	go func() {
		sub, err := l.client.Subscribe(ctx, fmt.Sprintf("%s%s", LockChannelPrefix, l.Name))
		if err != nil {
			c <- Sub{Err: err}
			return
		}
		for {
			select {
			case val := <-sub:
				switch v := val.(type) {
				case Subscription:
				case Message:
					c <- Sub{}
				case error:
					c <- Sub{Err: v}
					return
				}
			case <-ctx.Done():
				// c <- Sub{Err: ctx.Err()}
				return
			}
		}
	}()
	return c
}

func (l *Lock) Unlock(ctx context.Context) error {
	defer func() {
		if atomic.LoadUint32(&l.hasDog) == 1 {
			atomic.CompareAndSwapUint32(&l.hasDog, 1, 0)
			if l.doneC != nil {
				close(l.doneC)
			}
		}
	}()

	channelName := fmt.Sprintf("%s%s", LockChannelPrefix, l.Name)

	res := l.client.Eval(ctx, unlockScript, []string{l.Name, channelName}, l.Id, 1)

	if res.Err != nil {
		return res.Err
	}

	return nil

}

func (l *Lock) startWatchDog(ctx context.Context, expireTime time.Duration) {
	if l.doneC == nil {
		l.doneC = make(chan struct{})
	}
	go func(expire time.Duration) {

		defer func() {
			l.doneC = nil
		}()

		tick := time.Tick(expire / 3)
		for {
			select {
			case <-l.doneC:
				return
			case <-tick:
				if err := l.Expire(ctx, expire); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}

	}(expireTime)
}

func (l *Lock) Expire(ctx context.Context, expireTime time.Duration) error {

	res := l.client.Eval(ctx, expireScript, []string{l.Name}, l.Id, expireTime.Milliseconds())

	if res.Err != nil {
		return res.Err
	}

	if res.Val == nil {
		return errors.New("锁不存在")
	}

	return nil

}
