package redisson

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// check RedissonLock implements Lock
	_ Lock = (*RedissonLock)(nil)
)

var (
	// ErrObtainLockTimeout indicates that Lock cannot be acquired within waitTime
	ErrObtainLockTimeout = errors.New("obtained lock timeout")
)

// RedissonLock is a distributed lock implementation
type RedissonLock struct {
	RedissonBaseLock
}

// getChannelName returns the channel name for the lock
func (m *RedissonLock) getChannelName() string {
	return m.prefixName("redisson_lock__channel", m.getRawName())
}

// newRedisLock creates a new RedissonLock
func newRedisLock(name string, Redisson *Redisson) Lock {
	redisLock := &RedissonLock{}
	redisLock.RedissonBaseLock = *newBaseLock(Redisson.id, name, Redisson, redisLock)
	return redisLock
}

// tryLockInner tries to acquire the lock
func (m *RedissonLock) tryLockInner(ctx context.Context, leaseTime time.Duration, goroutineId uint64) (*int64, error) {
	result, err := m.client.Eval(ctx, `
if (redis.call('exists', KEYS[1]) == 0) then
    redis.call('hincrby', KEYS[1], ARGV[2], 1);
    redis.call('pexpire', KEYS[1], ARGV[1]);
    return nil;
end ;
if (redis.call('hexists', KEYS[1], ARGV[2]) == 1) then
    redis.call('hincrby', KEYS[1], ARGV[2], 1);
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

// unlockInner releases the lock
func (m *RedissonLock) unlockInner(ctx context.Context, goroutineId uint64) (*int64, error) {
	defer m.cancelExpirationRenewal(goroutineId)
	result, err := m.client.Eval(ctx, `
if (redis.call('hexists', KEYS[1], ARGV[3]) == 0) then
    return nil;
end ;
local counter = redis.call('hincrby', KEYS[1], ARGV[3], -1);
if (counter > 0) then
    redis.call('pexpire', KEYS[1], ARGV[2]);
    return 0;
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

// renewExpirationInner renews the lock expiration
func (m *RedissonLock) renewExpirationInner(ctx context.Context, goroutineId uint64) (int64, error) {
	return m.client.Eval(ctx, `
if (redis.call('hexists', KEYS[1], ARGV[2]) == 1) then
    redis.call('pexpire', KEYS[1], ARGV[1]);
    return 1;
end ;
return 0;
`, []string{m.getRawName()}, m.internalLockLeaseTime.Milliseconds(), m.getLockName(goroutineId)).Int64()
}
