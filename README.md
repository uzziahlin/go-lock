# go-lock

基于go语言的redis分布式锁实现，使用实例如下：
```
import (
    "context"
    redisv9 "github.com/redis/go-redis/v9"
    "github.com/uzziahlin/go-lock/redis"
)

var lock Lock

// 实例化redis的client，也可以自己实现redis.Client接口进行注入
client := redis.NewDefaultClient(&redisv9.Options{
    Addr:     "your redis address",
    Password: "you redis password",
    DB:       0,
})

// 获得一把名为lockName的分布式锁
lock = redis.NewLock("lockName", redis.WithClient(client))

// 可以通过context实现加锁超时控制
err := lock.Lock(context.TODO())

if err != nil {
    // todo 加锁失败
}

// todo 加锁成功
