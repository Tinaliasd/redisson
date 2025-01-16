package redisson

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedissonReadLock implements Lock
type RedissonReadLock struct {
	RedissonBaseLock
}

// newReadLock creates a new RedissonReadLock
func newReadLock(name string, redisson *Redisson) Lock {
	RedissonReadLock := &RedissonReadLock{}
	RedissonReadLock.RedissonBaseLock = *newBaseLock(redisson.id, name, redisson, RedissonReadLock)
	return RedissonReadLock
}

// getChannelName returns the channel name for the lock
func (m *RedissonReadLock) getChannelName() string {
	return m.prefixName("go_redisson_rwlock", m.getRawName())
}

// getWriteLockName returns the write lock name for the lock
func (m *RedissonReadLock) getWriteLockName(goroutineId uint64) string {
	return m.getLockName(goroutineId) + ":write"
}

// getReadWriteTimeoutNamePrefix returns the read write timeout name prefix for the lock
func (m *RedissonReadLock) getReadWriteTimeoutNamePrefix(goroutineId uint64) string {
	return m.suffixName(m.getRawName(), m.getLockName(goroutineId)) + ":rwlock_timeout"
}

// tryLockInner tries to acquire the lock
func (m *RedissonReadLock) tryLockInner(ctx context.Context, leaseTime time.Duration, goroutineId uint64) (*int64, error) {
	result, err := m.client.Eval(ctx, `
local mode = redis.call('hget', KEYS[1], 'mode');
if (mode == false) then
    redis.call('hset', KEYS[1], 'mode', 'read');
    redis.call('hset', KEYS[1], ARGV[2], 1);
    redis.call('set', KEYS[2] .. ':1', 1);
    redis.call('pexpire', KEYS[2] .. ':1', ARGV[1]);
    redis.call('pexpire', KEYS[1], ARGV[1]);
    return nil;
end ;
if (mode == 'read') or (mode == 'write' and redis.call('hexists', KEYS[1], ARGV[3]) == 1) then
    local ind = redis.call('hincrby', KEYS[1], ARGV[2], 1);
    local key = KEYS[2] .. ':' .. ind
    redis.call('set', key, 1);
    redis.call('pexpire', key, ARGV[1]);
    local remainTime = redis.call('pttl', KEYS[1]);
    redis.call('pexpire', KEYS[1], math.max(remainTime, ARGV[1]));
    return nil;
end ;
return redis.call('pttl', KEYS[1]);
`, []string{m.getRawName(), m.getReadWriteTimeoutNamePrefix(goroutineId)}, leaseTime.Milliseconds(),
		m.getLockName(goroutineId), m.getWriteLockName(goroutineId)).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &result, err
}

// getKeyPrefix returns the key prefix for the lock
func (m *RedissonReadLock) getKeyPrefix(goroutineId uint64, timeoutPrefix string) string {
	return strings.Split(timeoutPrefix, ":"+m.getLockName(goroutineId))[0]
}

// unlockInner unlocks the lock
func (m *RedissonReadLock) unlockInner(ctx context.Context, goroutineId uint64) (*int64, error) {
	defer m.cancelExpirationRenewal(goroutineId)

	timeoutPrefix := m.getReadWriteTimeoutNamePrefix(goroutineId)
	keyPrefix := m.getKeyPrefix(goroutineId, timeoutPrefix)

	result, err := m.client.Eval(ctx, `
local mode = redis.call('hget', KEYS[1], 'mode');
if (mode == false) then
    redis.call('publish', KEYS[2], ARGV[1]);
    return 1;
end ;
local lockExists = redis.call('hexists', KEYS[1], ARGV[2]);
if (lockExists == 0) then
    return nil;
end ;

local counter = redis.call('hincrby', KEYS[1], ARGV[2], -1);
if (counter == 0) then
    redis.call('hdel', KEYS[1], ARGV[2]);
end ;
redis.call('del', KEYS[3] .. ':' .. (counter + 1));

if (redis.call('hlen', KEYS[1]) > 1) then
    local maxRemainTime = -3;
    local keys = redis.call('hkeys', KEYS[1]);
    for n, key in ipairs(keys) do
        counter = tonumber(redis.call('hget', KEYS[1], key));
        if type(counter) == 'number' then
            for i = counter, 1, -1 do
                local remainTime = redis.call('pttl', KEYS[4] .. ':' .. key .. ':rwlock_timeout:' .. i);
                maxRemainTime = math.max(remainTime, maxRemainTime);
            end ;
        end ;
    end ;

    if maxRemainTime > 0 then
        redis.call('pexpire', KEYS[1], maxRemainTime);
        return 0;
    end ;

    if mode == 'write' then
        return 0;
    end ;
end ;

redis.call('del', KEYS[1]);
redis.call('publish', KEYS[2], ARGV[1]);
return 1;
`, []string{m.getRawName(), m.getChannelName(), timeoutPrefix, keyPrefix}, unlockMessage, m.getLockName(goroutineId)).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &result, err
}

// renewExpirationInner renews the expiration of the lock
func (m *RedissonReadLock) renewExpirationInner(ctx context.Context, goroutineId uint64) (int64, error) {
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
