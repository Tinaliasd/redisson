package redisson

import (
	"context"
	"github.com/redis/go-redis/v9"
)

type AtomicLong interface {
	RExpirable
	GetAndDecrement() (int64, error)
	AddAndGet(int642 int64) int64
	CompareAndSet(int64, int64) (bool, error)
	Get() (int64, error)
	GetAndDelete() (int64, error)
	GetAndAdd(int64) (int64, error)
	GetAndSet(int64) (int64, error)
	IncrementAndGet() int64
	GetAndIncrement() (int64, error)
	Set(int64) error
	DecrementAndGet() int64
}

type RedissonAtomicLong struct {
	*RedissonExpirable
}

var (
	_ AtomicLong = (*RedissonAtomicLong)(nil)
)

func NewRedissonAtomicLong(redisson *Redisson, name string) *RedissonAtomicLong {
	return &RedissonAtomicLong{
		RedissonExpirable: newRedissonExpirable(name, redisson),
	}
}

func (m *RedissonAtomicLong) AddAndGet(delta int64) int64 {
	return m.client.IncrBy(context.Background(), m.getRawName(), delta).Val()
}

func (m *RedissonAtomicLong) CompareAndSet(expect int64, update int64) (bool, error) {
	r, err := m.client.Eval(context.Background(), `
local currValue = redis.call('get', KEYS[1]);
if currValue == ARGV[1]
     or (tonumber(ARGV[1]) == 0 and currValue == false) then
 redis.call('set', KEYS[1], ARGV[2]);
 return 1
else
 return 0
end
`, []string{m.getRawName()}, expect, update).Int()
	if err != nil {
		return false, err
	}
	return r == 1, nil
}

func (m *RedissonAtomicLong) DecrementAndGet() int64 {
	return m.client.IncrBy(context.Background(), m.getRawName(), -1).Val()
}

func (m *RedissonAtomicLong) Get() (int64, error) {
	r, err := m.client.Get(context.Background(), m.getRawName()).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return r, err
}

func (m *RedissonAtomicLong) GetAndDelete() (int64, error) {
	r, err := m.client.Eval(context.Background(), `
local currValue = redis.call('get', KEYS[1]);
redis.call('del', KEYS[1]);
return currValue;
`, []string{m.getRawName()}, m.getRawName()).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return r, err
}

func (m *RedissonAtomicLong) GetAndAdd(delta int64) (int64, error) {
	v, err := m.client.Do(context.Background(), "INCRBY", m.getRawName(), delta).Int64()
	if err != nil {
		return 0, err
	}
	return v - delta, nil
}

func (m *RedissonAtomicLong) GetAndSet(newValue int64) (int64, error) {
	f, err := m.client.GetSet(context.Background(), m.getRawName(), newValue).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return f, err
}

func (m *RedissonAtomicLong) IncrementAndGet() int64 {
	return m.client.IncrBy(context.Background(), m.getRawName(), 1).Val()
}

func (m *RedissonAtomicLong) GetAndIncrement() (int64, error) {
	return m.GetAndAdd(1)
}

func (m *RedissonAtomicLong) GetAndDecrement() (int64, error) {
	return m.GetAndAdd(-1)
}

func (m *RedissonAtomicLong) Set(newValue int64) error {
	return m.client.Do(context.Background(), "SET", m.getRawName(), newValue).Err()
}
