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
	// DefaultExpireTime 锁默认的过期时间，如果用户没有为锁设置过期时间，则用此默认时间
	DefaultExpireTime = 30 * time.Second
)

// Lock 实现了核心接口Lock
// 基于redis的Lock实现，通过Hash类型实现了可重入锁
type Lock struct {
	// redis客户端，默认使用DefaultClient实现
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

	// 如果用户没有设置过期时间，则设置过期时间为默认过期时间
	if expireTime == 0 {
		expireTime = DefaultExpireTime.Milliseconds()
	}

	once := sync.Once{}

	var sub <-chan Sub

	for {
		// 执行加锁lua脚本，如果加锁成功，则返回-1，如果加锁不成功，则返回目前锁的过期时间ttl
		res := l.client.Eval(ctx, lockScript, []string{l.Name}, l.Id, expireTime)

		if res.Err != nil {
			return res.Err
		}

		val := res.Val.(int64)

		// 如果val == -1 说明已经拿到锁了
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
			// 订阅redis的channel，当用户释放锁时会往该channel写入消息通知其他人锁已经释放了
			sub = l.Subscribe(ctx)
		})

		// 有2种情况发生会重新进行抢锁：1.收到channel的释放锁通知 2.阻塞上边获取的redis里该锁的过期时间后
		// 如果设置了超时时间，则可能会超时
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

// Subscribe 订阅redis特定的channel，当channel有特定消息写入时会通过chan返回来
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

	// 执行解锁脚本，解锁是会判断Id和Key对应的field是否一致，防止误删锁
	// 如果解锁成功，则会往特定channel写入消息，通知其他用户该锁已经释放，可以重新抢锁
	res := l.client.Eval(ctx, unlockScript, []string{l.Name, channelName}, l.Id, 1)

	if res.Err != nil {
		return res.Err
	}

	return nil

}

// startWatchDog 当用户没有设置锁过期时间，会自动开启开门狗机制
// 默认情况下会给锁设置30秒的过期时间，并且三分之一过期时间时会定期检测锁是否还存在，如果存在则会进行锁续约
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

// Expire 重置锁的过期时间
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
