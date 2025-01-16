package redisson

// ReadWriteLock is a interface for read/write lock
type ReadWriteLock interface {
	ReadLock() Lock
	WriteLock() Lock
}

var (
	// check if RedissonReadWriteLock implements ReadWriteLock
	_ ReadWriteLock = (*RedissonReadWriteLock)(nil)
)

// RedissonReadWriteLock is the implementation of ReadWriteLock
type RedissonReadWriteLock struct {
	*RedissonExpirable
	redisson *Redisson
	rLock    Lock //the readLock instance
	wLock    Lock //the writeLock instance
}

// ReadLock return a readLock that can locks/unlocks for reading.
func (m *RedissonReadWriteLock) ReadLock() Lock {
	return m.rLock
}

// WriteLock return a writeLock that can locks/unlocks for writing.
func (m *RedissonReadWriteLock) WriteLock() Lock {
	return m.wLock
}

// newRedisReadWriteLock creates a new RedissonReadWriteLock
func newRedisReadWriteLock(name string, redisson *Redisson) ReadWriteLock {
	return &RedissonReadWriteLock{
		RedissonExpirable: newRedissonExpirable(name, redisson),
		redisson:          redisson,
		rLock:             newReadLock(name, redisson),
		wLock:             newRedisWriteLock(name, redisson),
	}
}
