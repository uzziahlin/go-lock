package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uzziahlin/go-lock"
	"sync"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	suite.Run(t, &ClientE2ESuite{})
}

type ClientE2ESuite struct {
	suite.Suite
	rdb Client
}

func (c *ClientE2ESuite) SetupSuite() {
	c.rdb = NewDefaultClient(&redis.Options{
		Addr:     "***.***.***.***:6379",
		Password: "",
		DB:       0,
	})
}

func (c *ClientE2ESuite) TestLock_Lock() {

	t := c.T()

	rdb := c.rdb

	testCases := []struct {
		name    string
		l       func() lock.Lock
		ctx     func() (context.Context, func())
		before  func(ctx context.Context, l lock.Lock)
		after   func(ctx context.Context, l lock.Lock)
		wantErr error
	}{
		{
			name: "测试上锁成功",
			l: func() lock.Lock {
				return NewLock("test_locked", WithClient(rdb))
			},
			ctx: func() (context.Context, func()) {
				return context.Background(), nil
			},
			before: func(ctx context.Context, l lock.Lock) {

			},
			after: func(ctx context.Context, l lock.Lock) {
				_ = l.Unlock(ctx)
			},
		},
		{
			name: "测试上锁超时",
			l: func() lock.Lock {
				return NewLock("test_lock_timeout", WithClient(rdb))
			},
			ctx: func() (context.Context, func()) {
				return context.WithTimeout(context.Background(), 3*time.Second)
			},
			before: func(ctx context.Context, l lock.Lock) {
				l = NewLock("test_lock_timeout", WithClient(rdb))
				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = l.Lock(ctx)
				}()
				wg.Wait()
			},
			after: func(ctx context.Context, l lock.Lock) {
				err := l.Unlock(context.Background())
				if err != nil {
					fmt.Println(err)
				}
			},
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "锁过期，抢锁成功",
			l: func() lock.Lock {
				return NewLock("test_lock_deadline", WithClient(rdb))
			},
			ctx: func() (context.Context, func()) {
				return context.WithTimeout(context.Background(), 5*time.Second)
			},
			before: func(ctx context.Context, l lock.Lock) {
				l = NewLock("test_lock_deadline", WithClient(rdb), WithExpire(3*time.Second))
				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = l.Lock(ctx)
				}()
				wg.Wait()
			},
			after: func(ctx context.Context, l lock.Lock) {
				err := l.Unlock(context.Background())
				if err != nil {
					fmt.Println(err)
				}
			},
		},
		{
			name: "主动释放锁，抢锁成功",
			l: func() lock.Lock {
				return NewLock("test_lock_release", WithClient(rdb))
			},
			ctx: func() (context.Context, func()) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			before: func(ctx context.Context, l lock.Lock) {
				l = NewLock("test_lock_release", WithClient(rdb), WithExpire(30*time.Second))
				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = l.Lock(ctx)
					time.AfterFunc(2*time.Second, func() {
						_ = l.Unlock(context.Background())
					})
				}()
				wg.Wait()
			},
			after: func(ctx context.Context, l lock.Lock) {
				err := l.Unlock(context.Background())
				if err != nil {
					fmt.Println(err)
				}
			},
		},
		{
			name: "测试看门狗机制",
			l: func() lock.Lock {
				return NewLock("test_lock_watch_dog", WithClient(rdb))
			},
			ctx: func() (context.Context, func()) {
				return context.WithTimeout(context.Background(), 10*time.Second)
			},
			before: func(ctx context.Context, l lock.Lock) {
				DefaultExpireTime = 5 * time.Second
				l = NewLock("test_lock_watch_dog", WithClient(rdb))
				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = l.Lock(ctx)
				}()
				wg.Wait()
			},
			after: func(ctx context.Context, l lock.Lock) {
				DefaultExpireTime = 30 * time.Second
				err := l.Unlock(context.Background())
				if err != nil {
					fmt.Println(err)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.ctx()
			defer func() {
				if cancel != nil {
					cancel()
				}
			}()
			l := tc.l()
			tc.before(ctx, l)
			err := l.Lock(ctx)
			tc.after(ctx, l)
			require.Equal(t, tc.wantErr, err)

		})
	}

}
