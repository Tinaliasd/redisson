package redisson

import (
	"context"
	"time"
)

// interface for inner locker
type innerLocker interface {
	tryLockInner(context.Context, time.Duration, uint64) (*int64, error)
	unlockInner(context.Context, uint64) (*int64, error)
	getChannelName() string
	renewExpirationInner(context.Context, uint64) (int64, error)
}

// A Lock represents an object that can be locked and unlocked.
type Lock interface {
	RExpirable
	Lock() error
	Unlock() error

	LockContext(context.Context) error
	UnlockContext(context.Context) error
}
