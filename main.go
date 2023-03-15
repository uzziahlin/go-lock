package lock

import (
	"context"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/uzziahlin/go-lock/redis"
)

func main() {
	var lock Lock
	client := redis.NewDefaultClient(&redisv9.Options{
		Addr:     "your redis address",
		Password: "you redis password",
		DB:       0,
	})
	lock = redis.NewLock("lockName", redis.WithClient(client))
	err := lock.Lock(context.TODO())
	if err != nil {
		// todo 加锁失败
	}
	// 加锁成功
}
