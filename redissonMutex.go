package redisson

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// check RedissonMutex implements Lock
	_ Lock = (*RedissonMutex)(nil)
)

// RedissonMutex is a distributed lock implementation
type RedissonMutex struct {
	RedissonBaseLock
}

// getChannelName returns the channel name for the mutex
func (m *RedissonMutex) getChannelName() string {
	return m.prefixName("redisson_mutex__channel", m.getRawName())
}

// newGoRedissonMutex creates a new RedissonMutex
func newRedissonMutex(name string, redisson *Redisson) *RedissonMutex {
	redisLock := &RedissonMutex{}
	redisLock.RedissonBaseLock = *newBaseLock(redisson.id, name, redisson, redisLock)
	return redisLock
}

// tryLockInner tries to acquire the mutex
func (m *RedissonMutex) tryLockInner(ctx context.Context, leaseTime time.Duration, goroutineId uint64) (*int64, error) {
	result, err := m.client.Eval(ctx, `
if (redis.call('setnx', KEYS[1], ARGV[2]) == 1) then
    redis.call('pexpire', KEYS[1], ARGV[1]);
    return nil;
end ;
return redis.call('pttl', KEYS[1]);
`, []string{m.getRawName()}, leaseTime.Milliseconds(), m.getLockName(goroutineId)).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &result, err
}

// unlockInner releases the mutex
func (m *RedissonMutex) unlockInner(ctx context.Context, goroutineId uint64) (*int64, error) {
	defer m.cancelExpirationRenewal(goroutineId)
	result, err := m.client.Eval(ctx, `
local val = redis.call('get', KEYS[1]);
if (val ~= ARGV[3]) then
    return nil;
else
    redis.call('del', KEYS[1]);
    redis.call('publish', KEYS[2], ARGV[1]);
    return 1;
end ;
return nil;
`, []string{m.getRawName(), m.getChannelName()}, unlockMessage, m.internalLockLeaseTime.Milliseconds(), m.getLockName(goroutineId)).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &result, err
}

// renewExpirationInner renews the mutex expiration
func (m *RedissonMutex) renewExpirationInner(ctx context.Context, goroutineId uint64) (int64, error) {
	return m.client.Eval(ctx, `
if (redis.call('exists', KEYS[1]) == 1) then
    redis.call('pexpire', KEYS[1], ARGV[1]);
    return 1;
end ;
return 0;
`, []string{m.getRawName()}, m.internalLockLeaseTime.Milliseconds(), m.getLockName(goroutineId)).Int64()
}
