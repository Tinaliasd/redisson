package redisson

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestReadWriteLock test read write lock
func TestReadWriteLock(t *testing.T) {
	l := GetRedisson().GetReadWriteLock("TestReadWriteLock")
	a := 0
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		innerWg := sync.WaitGroup{}
		for i := 0; i < 200; i++ {
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				err := l.WriteLock().LockContext(ctx)
				if err != nil {
					panic(err)
				}
				a++
				err = l.WriteLock().Unlock()
				if err != nil {
					panic(err)
				}
			}()
		}
		innerWg.Wait()
	}()

	go func() {
		defer wg.Done()
		innerWg := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				err := l.ReadLock().LockContext(ctx)
				if err != nil {
					panic(err)
				}
				err = l.ReadLock().Unlock()
				if err != nil {
					panic(err)
				}
			}()
		}
		innerWg.Wait()
	}()

	wg.Wait()
	if a != 200 {
		panic(a)
	}
}

// TestReadWriteLockFailFast test read write lock fail fast
func TestReadWriteLockFailFast(t *testing.T) {
	l := GetRedisson().GetReadWriteLock("TestReadWriteLockFailFast")
	a := 0
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		innerWg := sync.WaitGroup{}
		for i := 0; i < 500; i++ {
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				err := l.WriteLock().LockContext(ctx)
				if err != nil {
					return
				}
				a++
				err = l.WriteLock().Unlock()
				if err != nil {
					panic(err)
				}
			}()
		}
		innerWg.Wait()
	}()
	wg.Wait()
}

func TestRWMutex(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(-1))
	n := 100
	if testing.Short() {
		n = 5
	}
	HammerRWMutex(1, 1, n)
	HammerRWMutex(1, 3, n)
	HammerRWMutex(1, 10, n)
	HammerRWMutex(4, 1, n)
	HammerRWMutex(4, 3, n)
	HammerRWMutex(4, 10, n)
	HammerRWMutex(10, 1, n)
	HammerRWMutex(10, 3, n)
	HammerRWMutex(10, 10, n)
	HammerRWMutex(10, 5, n)
}

func writer(rwm ReadWriteLock, num_iterations int, activity *int32, cdone chan bool) {
	for i := 0; i < num_iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := rwm.WriteLock().LockContext(ctx)
		cancel()
		if err != nil {
			panic(err)
		}
		n := atomic.AddInt32(activity, 10000)
		if n != 10000 {
			err = rwm.WriteLock().Unlock()
			if err != nil {
				panic(err)
			}
			panic(fmt.Sprintf("wlock(%d)\n", n))
		}
		for i := 0; i < 100; i++ {
		}
		atomic.AddInt32(activity, -10000)
		err = rwm.WriteLock().Unlock()
		if err != nil {
			panic(err)
		}
	}
	cdone <- true
}

func reader(rwm ReadWriteLock, num_iterations int, activity *int32, cdone chan bool) {
	for i := 0; i < num_iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := rwm.ReadLock().LockContext(ctx)
		cancel()
		if err != nil {
			panic(err)
		}
		n := atomic.AddInt32(activity, 1)
		if n < 1 || n >= 10000 {
			err = rwm.ReadLock().Unlock()
			if err != nil {
				panic(err)
			}
			panic(fmt.Sprintf("wlock(%d)\n", n))
		}
		for i := 0; i < 100; i++ {
		}
		atomic.AddInt32(activity, -1)
		err = rwm.ReadLock().Unlock()
		if err != nil {
			panic(err)
		}
	}
	cdone <- true
}

func HammerRWMutex(gomaxprocs, numReaders, num_iterations int) {
	runtime.GOMAXPROCS(gomaxprocs)
	// Number of active readers + 10000 * number of active writers.
	var activity int32
	var rwm ReadWriteLock
	rwm = GetRedisson().GetReadWriteLock("HammerRWMutex")
	cdone := make(chan bool)
	go writer(rwm, num_iterations, &activity, cdone)
	var i int
	for i = 0; i < numReaders/2; i++ {
		go reader(rwm, num_iterations, &activity, cdone)
	}
	go writer(rwm, num_iterations, &activity, cdone)
	for ; i < numReaders; i++ {
		go reader(rwm, num_iterations, &activity, cdone)
	}
	// Wait for the 2 writers and all readers to finish.
	for i := 0; i < 2+numReaders; i++ {
		<-cdone
	}
}
