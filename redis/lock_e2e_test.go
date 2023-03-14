package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uzziahlin/go-lock"
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
		Addr:     "159.75.251.57:6381",
		Password: "123456",
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
				go func() {
					_ = l.Lock(ctx)
				}()
			},
			after: func(ctx context.Context, l lock.Lock) {
				err := l.Unlock(context.Background())
				if err != nil {
					fmt.Println(err)
				}
			},
			wantErr: context.DeadlineExceeded,
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
