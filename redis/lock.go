package redis

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed scripts/redis_lock.lua
var lockScript string

//go:embed scripts/redis_expire.lua
var expireScript string

//go:embed scripts/redis_unlock.lua
var unlockScript string

var (
	LockChannelPrefix = "Unlock:"
	DefaultExpireTime = 30 * time.Second
)

type Lock struct {
	// redis客户端
	client Client
	// 锁的唯一标识，防止误删锁
	Id string
	// 锁的名字
	Name string
	// 锁的过期时间，默认为30秒
	ExpireTime time.Duration
	// 表示是否开启了看门狗机制
	hasDog uint32
	// 通知看门狗退出
	doneC chan struct{}
}

type LockOption func(lock *Lock)

func WithClient(client Client) LockOption {
	return func(lock *Lock) {
		lock.client = client
	}
}

func WithExpire(expire time.Duration) LockOption {
	return func(lock *Lock) {
		lock.ExpireTime = expire
	}
}

func NewLock(name string, opts ...LockOption) *Lock {

	client := NewDefaultClient(&redis.Options{})

	res := &Lock{
		Name:   name,
		client: client,
		Id:     uuid.New().String(),
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

	once := sync.Once{}

	var sub <-chan Sub

	for {
		res := l.client.Eval(ctx, lockScript, []string{l.Name}, l.Id, expireTime)

		if res.Err != nil {
			return res.Err
		}

		val := res.Val.(int64)

		// 如果val == nil 说明已经拿到锁了
		if val == -1 {

			// 如果用户没有设置过期时间则开启看门狗机制
			if l.ExpireTime == 0 && atomic.LoadUint32(&l.hasDog) == 0 {
				if atomic.CompareAndSwapUint32(&l.hasDog, 0, 1) {
					l.startWatchDog(ctx, DefaultExpireTime)
				}
			}

			return nil
		}

		// 保证只订阅一次
		once.Do(func() {
			sub = l.Subscribe(ctx)
		})

		select {
		case v := <-sub:
			if v.Err != nil {
				return v.Err
			}
		case <-time.After(time.Duration(val) * time.Millisecond):
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
				case *Subscription:
				case *Message:
					c <- Sub{}
				case *Pong:
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
