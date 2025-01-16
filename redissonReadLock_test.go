package redisson

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestReadLockRenew(t *testing.T) {
	g := GetRedisson()
	mutex := g.GetReadWriteLock("TestReadLockRenew")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := mutex.ReadLock().LockContext(ctx)
	if err != nil {
		panic(err)
	}

	time.Sleep(35 * time.Second)
	err = mutex.ReadLock().Unlock()
	if err != nil {
		panic(err)
	}

}

func TestWWLockUnlock(t *testing.T) {
	g := GetRedisson()
	var wl ReadWriteLock
	wl = g.GetReadWriteLock("TestWWLockUnlock")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := wl.WriteLock().LockContext(ctx)

		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
		err = wl.WriteLock().Unlock()
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(1 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	err := wl.WriteLock().LockContext(ctx)
	if err != nil {
		panic(err)
	}
	err = wl.WriteLock().Unlock()

	if err != nil {
		panic(err)
	}
}

func TestWRLockUnlock(t *testing.T) {
	g := GetRedisson()
	var wl ReadWriteLock
	wl = g.GetReadWriteLock("TestWRLockUnlock")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := wl.ReadLock().LockContext(ctx)
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
		err = wl.ReadLock().Unlock()
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(1 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	err := wl.WriteLock().LockContext(ctx)
	if err != nil {
		panic(err)
	}
	err = wl.WriteLock().Unlock()

	if err != nil {
		panic(err)
	}
}

func TestRWLockUnlock(t *testing.T) {
	g := GetRedisson()
	var wl ReadWriteLock
	wl = g.GetReadWriteLock("TestRWLockUnlock")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := wl.WriteLock().LockContext(ctx)

		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
		err = wl.WriteLock().Unlock()
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(1 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	err := wl.ReadLock().LockContext(ctx)
	if err != nil {
		panic(err)
	}
	err = wl.ReadLock().Unlock()

	if err != nil {
		panic(err)
	}
}

func TestRRLock(t *testing.T) {
	g := GetRedisson()
	var wl ReadWriteLock
	wl = g.GetReadWriteLock("TestRRLock")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := wl.ReadLock().LockContext(ctx)
	cancel()
	if err != nil {
		panic(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	err = wl.ReadLock().LockContext(ctx)
	cancel()
	if err != nil {
		panic("err")
	}
	err = wl.ReadLock().Unlock()
	if err != nil {
		panic(err)
	}
	err = wl.ReadLock().Unlock()
	if err != nil {
		panic(err)
	}
}

func TestReadLock(t *testing.T) {
	g := GetRedisson()
	key := strconv.FormatInt(int64(rand.Int31n(1000000)), 10)
	l := g.GetReadWriteLock(key)
	innerWg := sync.WaitGroup{}
	for i := 0; i < 500; i++ {
		innerWg.Add(1)
		go func() {
			defer innerWg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := l.ReadLock().LockContext(ctx)
			if err != nil {
				panic(err.Error() + ":" + key)
			}
			err = l.ReadLock().Unlock()
			if err != nil {
				panic(err.Error() + ":" + key)
			}
		}()
	}
	innerWg.Wait()
}
