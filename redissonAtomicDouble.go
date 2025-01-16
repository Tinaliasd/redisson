package redisson

import (
	"context"
	"github.com/redis/go-redis/v9"
	"strconv"
)

type AtomicDouble interface {
	RExpirable
	GetAndDecrement() (float64, error)
	AddAndGet(float642 float64) float64
	CompareAndSet(float64, float64) (bool, error)
	Get() (float64, error)
	GetAndDelete() (float64, error)
	GetAndAdd(float64) (float64, error)
	GetAndSet(float64) (float64, error)
	IncrementAndGet() float64
	GetAndIncrement() (float64, error)
	Set(float64) error
	DecrementAndGet() float64
}

var (
	_ AtomicDouble = (*RedissonAtomicDouble)(nil)
)

type RedissonAtomicDouble struct {
	*RedissonExpirable
}

func NewRedissonAtomicDouble(redisson *Redisson, name string) *RedissonAtomicDouble {
	return &RedissonAtomicDouble{
		RedissonExpirable: newRedissonExpirable(name, redisson),
	}
}

func (m *RedissonAtomicDouble) AddAndGet(delta float64) float64 {
	return m.client.IncrByFloat(context.Background(), m.getRawName(), delta).Val()
}

func (m *RedissonAtomicDouble) CompareAndSet(expect float64, update float64) (bool, error) {
	r, err := m.client.Eval(context.Background(), `
local value = redis.call('get', KEYS[1]);
if (value == false and tonumber(ARGV[1]) == 0) or (tonumber(value) == tonumber(ARGV[1])) then
     redis.call('set', KEYS[1], ARGV[2]);
     return 1
   else
return 0 end
`, []string{m.getRawName()}, strconv.FormatFloat(expect, 'e', -1, 64), strconv.FormatFloat(update, 'e', -1, 64)).Int()
	if err != nil {
		return false, err
	}
	return r == 1, nil
}

func (m *RedissonAtomicDouble) DecrementAndGet() float64 {
	return m.client.IncrByFloat(context.Background(), m.getRawName(), -1).Val()
}

func (m *RedissonAtomicDouble) Get() (float64, error) {
	r, err := m.client.Get(context.Background(), m.getRawName()).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return r, err
}

func (m *RedissonAtomicDouble) GetAndDelete() (float64, error) {
	r, err := m.client.Eval(context.Background(), `
local currValue = redis.call('get', KEYS[1]);
redis.call('del', KEYS[1]);
return currValue;
`, []string{m.getRawName()}, m.getRawName()).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return r, err
}

func (m *RedissonAtomicDouble) GetAndAdd(delta float64) (float64, error) {
	v, err := m.client.Do(context.Background(), "INCRBYFLOAT", m.getRawName(), delta).Float64()
	if err != nil {
		return 0, err
	}
	return v - delta, nil
}

func (m *RedissonAtomicDouble) GetAndSet(newValue float64) (float64, error) {
	f, err := m.client.GetSet(context.Background(), m.getRawName(), strconv.FormatFloat(newValue, 'e', -1, 64)).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return f, err
}

func (m *RedissonAtomicDouble) IncrementAndGet() float64 {
	return m.client.IncrByFloat(context.Background(), m.getRawName(), 1).Val()
}

func (m *RedissonAtomicDouble) GetAndIncrement() (float64, error) {
	return m.GetAndAdd(1)
}

func (m *RedissonAtomicDouble) GetAndDecrement() (float64, error) {
	return m.GetAndAdd(-1)
}

func (m *RedissonAtomicDouble) Set(newValue float64) error {
	return m.client.Do(context.Background(), "SET", m.getRawName(), strconv.FormatFloat(newValue, 'e', -1, 64)).Err()
}
