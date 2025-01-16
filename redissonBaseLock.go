package redisson

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/elliotchance/orderedmap/v2"
)

const (
	// unlockMessage is the message sent to the channel when the lock is unlocked
	unlockMessage int64 = 0
	// readUnlockMessage is the message sent to the channel when the lock is unlocked for read
	readUnlockMessage int64 = 1
)

// expirationEntry is a struct that holds the goroutine ids that are waiting for the lock to expire
type expirationEntry struct {
	//mutex is used to protect the following fields
	sync.Mutex
	// goroutineIds is a map of goroutine ids that are waiting for the lock to expire
	goroutineIds *orderedmap.OrderedMap[uint64, int64]
	// cancelFunc is the cancel function for the context that is used to cancel the goroutine that is waiting for the lock to expire
	cancelFunc context.CancelFunc
}

// newRenewEntry creates a new expirationEntry
func newRenewEntry() *expirationEntry {
	return &expirationEntry{
		goroutineIds: orderedmap.NewOrderedMap[uint64, int64](),
	}
}

// addGoroutineId adds a goroutine id to the expirationEntry
func (e *expirationEntry) addGoroutineId(goroutineId uint64) {
	e.Lock()
	defer e.Unlock()
	count, ok := e.goroutineIds.Get(goroutineId)
	if ok {
		count++
	} else {
		count = 1
	}
	e.goroutineIds.Set(goroutineId, count)
}

// removeGoroutineId removes a goroutine id from the expirationEntry
func (e *expirationEntry) removeGoroutineId(goroutineId uint64) {
	e.Lock()
	defer e.Unlock()

	count, ok := e.goroutineIds.Get(goroutineId)
	if !ok {
		return
	}
	count--
	if count == 0 {
		e.goroutineIds.Delete(goroutineId)
	} else {
		e.goroutineIds.Set(goroutineId, count)
	}
}

// hasNoThreads returns true if there are no goroutines waiting for the lock to expire
func (e *expirationEntry) hasNoThreads() bool {
	e.Lock()
	defer e.Unlock()
	return e.goroutineIds.Len() == 0
}

// getFirstGoroutineId returns the first goroutine id in the expirationEntry
func (e *expirationEntry) getFirstGoroutineId() *uint64 {
	e.Lock()
	defer e.Unlock()
	if e.goroutineIds.Len() == 0 {
		return nil
	}
	first := e.goroutineIds.Front().Key
	return &first
}

// RedissonBaseLock is the base lock struct
type RedissonBaseLock struct {
	*RedissonExpirable
	//expirationRenewal is a map of expiration entries that are used to renew the lock expiration
	ExpirationRenewalMap sync.Map
	//internalLockLeaseTime is the internal lock lease time
	//when the lock is acquired, the expiration is set to this value
	internalLockLeaseTime time.Duration
	id                    string
	entryName             string
	lock                  innerLocker
}

// newBaseLock creates a new RedissonBaseLock
func newBaseLock(key, name string, redisson *Redisson, locker innerLocker) *RedissonBaseLock {
	baseLock := &RedissonBaseLock{
		RedissonExpirable:     newRedissonExpirable(name, redisson),
		internalLockLeaseTime: redisson.watchDogTimeout,
		id:                    key,
		lock:                  locker,
	}
	baseLock.entryName = baseLock.id + ":" + name
	return baseLock
}

// getLockName returns the lock name
func (m *RedissonBaseLock) getLockName(goroutineId uint64) string {
	return m.id + ":" + strconv.FormatUint(goroutineId, 10)
}

// getEntryName returns the entry name
func (m *RedissonBaseLock) getEntryName() string {
	return m.entryName
}

// tryAcquire tries to acquire the lock
func (m *RedissonBaseLock) tryAcquire(ctx context.Context, goroutineId uint64) (*int64, error) {
	ttl, err := m.lock.tryLockInner(ctx, m.internalLockLeaseTime, goroutineId)
	if err != nil {
		return nil, err
	}
	// lock acquired
	if ttl == nil {
		m.scheduleExpirationRenewal(goroutineId)
	}
	return ttl, nil
}

// scheduleExpirationRenewal schedules the expiration renewal
func (m *RedissonBaseLock) scheduleExpirationRenewal(goroutineId uint64) {
	entry := newRenewEntry()
	oldEntry, stored := m.ExpirationRenewalMap.LoadOrStore(m.getEntryName(), entry)
	if stored {
		oldEntry.(*expirationEntry).addGoroutineId(goroutineId)
	} else {
		entry.addGoroutineId(goroutineId)
		m.renewExpiration()
	}
}

// renewExpiration renews the expiration
func (m *RedissonBaseLock) renewExpiration() {
	entryName := m.getEntryName()
	ee, ok := m.ExpirationRenewalMap.Load(entryName)
	if !ok {
		return
	}
	ti := time.NewTimer(m.internalLockLeaseTime / 3)

	ctx, cancel := context.WithCancel(context.Background())

	go func(ctx context.Context) {
		select {
		case <-ti.C:
			ent, ok := m.ExpirationRenewalMap.Load(entryName)
			if !ok {
				return
			}
			goroutineId := ent.(*expirationEntry).getFirstGoroutineId()
			if goroutineId == nil {
				return
			}
			res, err := m.lock.renewExpirationInner(ctx, *goroutineId)
			if err != nil {
				m.ExpirationRenewalMap.Delete(entryName)
				return
			}
			if res != 0 {
				m.renewExpiration()
				return
			}
			m.cancelExpirationRenewal(0)
			return
		case <-ctx.Done():
			return
		}
	}(ctx)
	ee.(*expirationEntry).Lock()
	ee.(*expirationEntry).cancelFunc = cancel
	ee.(*expirationEntry).Unlock()
}

// cancelExpirationRenewal cancels the expiration renewal
func (m *RedissonBaseLock) cancelExpirationRenewal(goroutineId uint64) {
	entry, ok := m.ExpirationRenewalMap.Load(m.getEntryName())
	if !ok {
		return
	}
	task := entry.(*expirationEntry)
	if goroutineId != 0 {
		task.removeGoroutineId(goroutineId)
	}
	if goroutineId == 0 || task.hasNoThreads() {
		task.Lock()
		if task.cancelFunc != nil {
			task.cancelFunc()
			task.cancelFunc = nil
		}
		task.Unlock()
		m.ExpirationRenewalMap.Delete(m.getEntryName())
	}
}

// Lock locks m. Lock returns when locking is successful or when an exception is encountered.
// use context.Background() to block until the lock is obtained
func (m *RedissonBaseLock) Lock() error {
	return m.LockContext(context.Background())
}

// LockContext locks m. Lock Returns when locking is successful or when the context timeout or an exception is encountered.
func (m *RedissonBaseLock) LockContext(ctx context.Context) error {
	goroutineId, err := getId()
	if err != nil {
		return err
	}
	// PubSub
	sub := m.client.Subscribe(ctx, m.lock.getChannelName())
	defer sub.Close()
	defer sub.Unsubscribe(context.TODO(), m.lock.getChannelName())
	ttl := new(int64)
	// fire
	// setting ttl to 0 will allow the for loop to start properly
	*ttl = 0
	for {
		select {
		// obtain lock timeout
		case <-ctx.Done():
			return ErrObtainLockTimeout
		// indicates that the lock has ttl milliseconds to expire
		// if the lock is not released within ttl milliseconds, the lock will expire
		// we need to try to acquire the lock again
		case <-time.After(time.Duration(*ttl) * time.Millisecond):
			ttl, err = m.tryAcquire(ctx, goroutineId)
		// a lock has been released
		// we need to try to acquire the lock again
		case <-sub.Channel():
			ttl, err = m.tryAcquire(ctx, goroutineId)
		}
		if err != nil {
			return err
		}
		// lock acquired
		if ttl == nil {
			return nil
		}
	}
}

// Unlock unlocks m. Unlock returns when unlocking is successful or when an exception is encountered.
func (m *RedissonBaseLock) Unlock() error {
	return m.UnlockContext(context.Background())
}

// UnlockContext unlocks m. UnlockContext Returns when unlocking is successful or when the context timeout or an exception is encountered.
func (m *RedissonBaseLock) UnlockContext(ctx context.Context) error {
	goroutineId, err := getId()
	if err != nil {
		return err
	}
	opStatus, err := m.lock.unlockInner(ctx, goroutineId)
	if err != nil {
		return err
	}
	if opStatus == nil {
		return fmt.Errorf("attempt to unlock lock, not locked by current goroutine by node id: %s goroutine-id: %d", m.id, goroutineId)
	}
	return nil
}
