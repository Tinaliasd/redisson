package redisson

import (
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/satori/go.uuid"
)

type RedissonConfig struct {
	//client redis client
	client *redis.Client
	//watchDogTimeout timeout for watchdog
	watchDogTimeout time.Duration
}

// Redisson is a redisson client.
type Redisson struct {
	RedissonConfig
	//id Redisson unique uuid
	id string
}

// DefaultWatchDogTimeout
// The default watchdog timeout, the watchdog will go every 1/3 of the DefaultWatchDogTimeout to renew the lock held by the current goroutine.
var DefaultWatchDogTimeout = 30 * time.Second

// NewRedisson returns a new Redisson instance.
func NewRedisson(redisClient *redis.Client, opts ...OptionFunc) *Redisson {
	g := &Redisson{
		RedissonConfig: RedissonConfig{
			client:          redisClient,
			watchDogTimeout: DefaultWatchDogTimeout,
		},
		id: uuid.NewV4().String(),
	}

	fmt.Println("NewRedisson id:", g.id)

	for _, opt := range opts {
		opt(g)
	}
	return g
}

// OptionFunc is a function that can be used to configure a Redisson instance.
type OptionFunc func(g *Redisson)

// WithWatchDogTimeout sets the timeout for the watchdog.
func WithWatchDogTimeout(t time.Duration) OptionFunc {
	return func(g *Redisson) {
		if t.Seconds() < 30 {
			t = DefaultWatchDogTimeout
			log.Println("watchDogTimeout is too small, so config default ")
		}
		g.watchDogTimeout = t
	}
}

// GetLock returns a Lock named "key" which can be used to lock and unlock the resource "key".
// A Lock can be copied after first use, but most of the time it is advisable to keep instances of Lock.
func (g *Redisson) GetLock(key string) Lock {
	return newRedisLock(key, g)
}

// GetReadWriteLock returns a ReadWriteLock named "key" which can be used to lock and unlock the resource "key" when reading or writing.
// A ReadWriteLock can be copied after first use, but most of the time it is advisable to keep instances of ReadWriteLock.
func (g *Redisson) GetReadWriteLock(key string) ReadWriteLock {
	return newRedisReadWriteLock(key, g)
}

// GetMutex returns a Mutex named "key" which can be used to lock and unlock the resource "key".
// A Mutex can be copied after first use, but most of the time it is advisable to keep instances of Lock.
// the difference between Mutex and Lock is that Lock can be locked multiple times by the same goroutine, but Mutex can only be locked once.
// the mutex is like sync.Mutex in golang.
//
// In the terminology of the Go memory model,
// the n'th call to Unlock “synchronizes before” the m'th call to Lock
// for any n < m.
func (g *Redisson) GetMutex(key string) Lock {
	return newRedissonMutex(key, g)
}

func (g *Redisson) GetRateLimiter(name string) RRateLimiter {
	return newRedissonRateLimiter(name, g)

}

func (g *Redisson) GetAtomicLong(key string) AtomicLong {
	return NewRedissonAtomicLong(g, key)
}
func (g *Redisson) GetAtomicDouble(key string) AtomicDouble {
	return NewRedissonAtomicDouble(g, key)
}

func (g *Redisson) GetBitSet(key string) BitSet {
	return NewRedissonBitSet(g, key)
}

// GetBloomFilter returns a new RBloomFilter instance
func GetBloomFilter[T any](r *Redisson, key string) RBloomFilter[T] {
	return NewRedissonBloomFilter[T](r, key)
}
