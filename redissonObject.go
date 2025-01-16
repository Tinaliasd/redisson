package redisson

import (
	"context"
	"github.com/redis/go-redis/v9"
	"strings"
	"sync"
)

// RedissonObject is the base struct for all objects
type RedissonObject struct {
	name string
	*Redisson
	mutex sync.Mutex
}

// prefixName prefixes the name with the given prefix
func (o *RedissonObject) prefixName(prefix string, name string) string {
	if strings.Contains(name, "{") {
		return prefix + ":" + name
	}
	return prefix + ":{" + name + "}"
}

// suffixName suffixes the name with the given suffix
func (o *RedissonObject) suffixName(name, suffix string) string {
	if strings.Contains(name, "{") {
		return name + ":" + suffix
	}
	return "{" + name + "}:" + suffix
}

// getRawName returns the raw name
func (o *RedissonObject) getRawName() string {
	return o.name
}

func newRedissonObjectNULL(name string) *RedissonObject {
	return &RedissonObject{
		name: name,
	}
}

// newRedissonObject creates a new RedissonObject
func newRedissonObject(name string, redisson *Redisson) *RedissonObject {
	return &RedissonObject{
		name:     name,
		Redisson: redisson,
	}
}

// sizeInMemoryAsync calculates the total memory usage for the given keys asynchronously
func (r *Redisson) sizeInMemoryAsync(keys []string) (*int64, error) {
	luaScript := `
		local total = 0;
		for j = 1, #KEYS, 1 do 
			local size = redis.call('memory', 'usage', KEYS[j]); 
			if size ~= false then 
				total = total + size;
			end; 
		end; 
		return total;`

	// Execute the Lua script
	//创建一个 ctx
	ctx := context.Background()
	res, err := r.client.Eval(ctx, luaScript, keys).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	return &res, nil
}
