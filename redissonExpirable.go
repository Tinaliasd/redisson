package redisson

import (
	"context"
	"time"
)

type RExpirable interface {
	// Expire sets an expiration duration for this object.
	Expire(duration time.Duration) (bool, error)

	// ExpireAt sets an expiration date for this object.
	ExpireAt(timestamp time.Time) (bool, error)

	// ClearExpire clears the expiration for this object.
	ClearExpire() (bool, error)

	// RemainTimeToLive returns the remaining time to live of the object in milliseconds.
	RemainTimeToLive() (int64, error)

	// GetExpireTime returns the expiration time of the object.
	GetExpireTime() (int64, error)

	TTL(key string) (time.Duration, error)
}

// RedissonExpirable is the base struct for all expirable objects
type RedissonExpirable struct {
	*RedissonObject
}

// newRedissonExpirable creates a new RedissonExpirable
func newRedissonExpirable(name string, redisson *Redisson) *RedissonExpirable {
	return &RedissonExpirable{
		RedissonObject: newRedissonObject(name, redisson),
	}
}

func (rep *RedissonExpirable) ExpireAt(t time.Time) (bool, error) {
	// Convert to Unix time in milliseconds
	timestamp := t.UnixNano() / 1e6
	// param can be an extra argument if needed, here we use empty string
	param := ""
	// Evaluate the Lua script
	ctx := context.Background()
	res, err := rep.client.Eval(ctx, expireAtLuaScript, []string{rep.getRawName()}, timestamp, param).Int64()
	if err != nil {
		return false, err
	}
	// Check if at least one key was successfully set to expire
	return res == 1, nil
}

// expire(Duration duration) - Sets expiration based on Duration
func (rep *RedissonExpirable) Expire(d time.Duration) (bool, error) {
	// Convert duration to milliseconds
	ms := d.Milliseconds()
	param := ""

	// Evaluate the Lua script
	ctx := context.Background()
	res, err := rep.client.Eval(ctx, expireLuaScript, []string{rep.getRawName()}, ms, param).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// clearExpire() - Removes any expiration from the key
func (rep *RedissonExpirable) ClearExpire() (bool, error) {

	ctx := context.Background()
	res, err := rep.client.Eval(ctx, clearExpireLuaScript, []string{rep.getRawName()}).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// remainTimeToLive() - Returns the remaining TTL in milliseconds
func (rep *RedissonExpirable) RemainTimeToLive() (int64, error) {

	ctx := context.Background()

	ttl, err := rep.client.PTTL(ctx, rep.getRawName()).Result()
	if err != nil {
		return 0, err
	}
	// If no expiration is set, PTTL may return -1
	return ttl.Milliseconds(), nil
}

// getExpireTime() - Returns the absolute expire time (Unix ms), or -1 if none
func (rep *RedissonExpirable) GetExpireTime() (int64, error) {

	ttl, err := rep.RemainTimeToLive()
	if err != nil {
		return -1, err
	}
	if ttl < 0 {
		// No expiration set
		return -1, nil
	}
	// Current time in milliseconds + remaining TTL
	return (time.Now().UnixNano()/1e6 + ttl), nil
}

// TTL 获取键的剩余过期时间
func (re *RedissonExpirable) TTL(key string) (time.Duration, error) {
	duration, err := re.client.TTL(context.Background(), key).Result()
	return duration, err
}

// Lua scripts separated from method definitions:

// expireLuaScript attempts to set a PEXPIRE for given keys.
const expireLuaScript = `
local result = 0;
for j = 1, #KEYS, 1 do
    local expireSet;
    if ARGV[2] ~= '' then
        expireSet = redis.call('pexpire', KEYS[j], ARGV[1], ARGV[2]);
    else
        expireSet = redis.call('pexpire', KEYS[j], ARGV[1]);
    end;
    if expireSet == 1 then
        result = expireSet;
    end;
end;
return result;
`

// expireAtLuaScript attempts to set a PEXPIREAT for given keys.
const expireAtLuaScript = `
local result = 0;
for j = 1, #KEYS, 1 do
    local expireSet;
    if ARGV[2] ~= '' then
        expireSet = redis.call('pexpireat', KEYS[j], ARGV[1], ARGV[2]);
    else
        expireSet = redis.call('pexpireat', KEYS[j], ARGV[1]);
    end;
    if expireSet == 1 then
        result = expireSet;
    end;
end;
return result;
`

// clearExpireLuaScript removes expiration from given keys (PERSIST).
const clearExpireLuaScript = `
local result = 0;
for j = 1, #KEYS, 1 do
    local expireSet = redis.call('persist', KEYS[j]);
    if expireSet == 1 then
        result = expireSet;
    end;
end;
return result;
`
