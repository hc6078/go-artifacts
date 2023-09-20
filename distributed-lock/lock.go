package distributed_lock

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

// 分布式锁

var Redis = redis.NewClient(&redis.Options{
	Addr:     "localhost:6379",
	Password: "", // no password set
	DB:       0,  // use default DB
})

// 加锁要有超时时间
func lock(key, uuid string, expiration time.Duration) (bool, error) {
	return Redis.SetNX(context.Background(), key, uuid, expiration).Result()
}

// 解锁要判断锁的持有者与加锁者是否一致
func unLock(key string, uuid string) (interface{}, error) {
	var deleteScript = `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return
		end
	`
	return Redis.Eval(context.Background(), deleteScript, []string{key}, uuid).Result()
}
