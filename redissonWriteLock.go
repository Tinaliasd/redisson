package redisson

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// redissonWriteLock implements Lock
type redissonWriteLock struct {
	RedissonBaseLock
}

// getChannelName returns the channel name for the lock
func (m *redissonWriteLock) getChannelName() string {
	return m.prefixName("go_redisson_rwlock", m.getRawName())
}

// getLockName returns the lock name for the lock
func (m *redissonWriteLock) getLockName(goroutineId uint64) string {
	return m.RedissonBaseLock.getLockName(goroutineId) + ":write"
}

// newRedisWriteLock creates a new redissonWriteLock
func newRedisWriteLock(name string, redisson *Redisson) Lock {
	redisWriteLock := &redissonWriteLock{}
	redisWriteLock.RedissonBaseLock = *newBaseLock(redisson.id, name, redisson, redisWriteLock)
	return redisWriteLock
}

// tryLockInner tries to acquire the lock
func (m *redissonWriteLock) tryLockInner(ctx context.Context, leaseTime time.Duration, goroutineId uint64) (*int64, error) {
	result, err := m.client.Eval(ctx, `
local mode = redis.call('hget', KEYS[1], 'mode');
if (mode == false) then
    redis.call('hset', KEYS[1], 'mode', 'write');
    redis.call('hset', KEYS[1], ARGV[2], 1);
    redis.call('pexpire', KEYS[1], ARGV[1]);
    return nil;
end ;
if (mode == 'write') then
    if (redis.call('hexists', KEYS[1], ARGV[2]) == 1) then
        redis.call('hincrby', KEYS[1], ARGV[2], 1);
        local currentExpire = redis.call('pttl', KEYS[1]);
        redis.call('pexpire', KEYS[1], currentExpire + ARGV[1]);
        return nil;
    end ;
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

// unlockInner unlocks the lock
func (m *redissonWriteLock) unlockInner(ctx context.Context, goroutineId uint64) (*int64, error) {
	defer m.cancelExpirationRenewal(goroutineId)

	result, err := m.client.Eval(ctx, `
local mode = redis.call('hget', KEYS[1], 'mode');
if (mode == false) then
    redis.call('publish', KEYS[2], ARGV[1]);
    return 1;
end ;
if (mode == 'write') then
    local lockExists = redis.call('hexists', KEYS[1], ARGV[3]);
    if (lockExists == 0) then
        return nil;
    else
        local counter = redis.call('hincrby', KEYS[1], ARGV[3], -1);
        if (counter > 0) then
            redis.call('pexpire', KEYS[1], ARGV[2]);
            return 0;
        else
            redis.call('hdel', KEYS[1], ARGV[3]);
            if (redis.call('hlen', KEYS[1]) == 1) then
                redis.call('del', KEYS[1]);
                redis.call('publish', KEYS[2], ARGV[1]);
            else
                redis.call('hset', KEYS[1], 'mode', 'read');
            end ;
            return 1;
        end ;
    end ;
end ;
return nil;
`, []string{m.getRawName(), m.getChannelName()}, readUnlockMessage, m.internalLockLeaseTime.Milliseconds(), m.getLockName(goroutineId)).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &result, err
}

// getKeyPrefix returns the key prefix for the lock
func (m *redissonWriteLock) getKeyPrefix(goroutineId uint64, timeoutPrefix string) string {
	return strings.Split(timeoutPrefix, ":"+m.getLockName(goroutineId))[0]
}

// getReadWriteTimeoutNamePrefix returns the read write timeout name prefix for the lock
func (m *redissonWriteLock) getReadWriteTimeoutNamePrefix(goroutineId uint64) string {
	return m.suffixName(m.getRawName(), m.getLockName(goroutineId)) + ":rwlock_timeout"
}

// renewExpirationInner renews the expiration of the lock
func (m *redissonWriteLock) renewExpirationInner(ctx context.Context, goroutineId uint64) (int64, error) {
	timeoutPrefix := m.getReadWriteTimeoutNamePrefix(goroutineId)
	keyPrefix := m.getKeyPrefix(goroutineId, timeoutPrefix)

	return m.client.Eval(ctx, `
local counter = redis.call('hget', KEYS[1], ARGV[2]);
if (counter ~= false) then
    redis.call('pexpire', KEYS[1], ARGV[1]);

    if (redis.call('hlen', KEYS[1]) > 1) then
        local keys = redis.call('hkeys', KEYS[1]);
        for n, key in ipairs(keys) do
            counter = tonumber(redis.call('hget', KEYS[1], key));
            if type(counter) == 'number' then
                for i = counter, 1, -1 do
                    redis.call('pexpire', KEYS[2] .. ':' .. key .. ':rwlock_timeout:' .. i, ARGV[1]);
                end ;
            end ;
        end ;
    end ;

    return 1;
end ;
return 0;
`, []string{m.getRawName(), keyPrefix}, m.internalLockLeaseTime.Milliseconds(), m.getLockName(goroutineId)).Int64()
}
