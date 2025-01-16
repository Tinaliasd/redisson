package redisson

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisAddr = "localhost:6379"
)

// GetRedisson get a Redisson  instance
func GetRedisson() *Redisson {
	redisDB := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return NewRedisson(redisDB)
}

// TestLockRenew test lock renew
func TestLockRenew(t *testing.T) {
	g := GetRedisson()
	lock := g.GetLock("TestMutexRenew")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := lock.LockContext(ctx)
	if err != nil {
		panic(err)
	}

	time.Sleep(35 * time.Second)
	err = lock.Unlock()
	if err != nil {
		panic(err)
	}
}

// TestLockRenewTwice test lock renew twice
func TestLockRenewTwice(t *testing.T) {
	g := GetRedisson()
	lock := g.GetLock("TestMutexRenew")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := lock.LockContext(ctx)
	if err != nil {
		panic(err)
	}

	time.Sleep(45 * time.Second)
	err = lock.Unlock()
	if err != nil {
		panic(err)
	}

}

// TestLockRenewTogether test lock renew together
func TestLockRenewTogether(t *testing.T) {
	g := GetRedisson()
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		ind := i
		go func() {
			defer wg.Done()
			lock := g.GetLock("TestMutexRenew" + strconv.Itoa(ind))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := lock.LockContext(ctx)
			if err != nil {
				panic(err)
			}
			time.Sleep(35 * time.Second)
			err = lock.Unlock()
			if err != nil {
				panic(err)
			}
		}()
	}
	wg.Wait()
}

// TestWithWatchDogTimeout test with watchdog timeout
func TestWithWatchDogTimeout(t *testing.T) {
	redisDB := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	g := NewRedisson(redisDB, WithWatchDogTimeout(time.Second*39))
	lock := g.GetLock("TestMutexRenew")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := lock.LockContext(ctx)
	if err != nil {
		panic(err)
	}

	time.Sleep(15 * time.Second)
	err = lock.Unlock()
	if err != nil {
		panic(err)
	}

}

// TestWithWatchDogTimeout2 test with small watchdog timeout
func TestWithWatchDogTimeout2(t *testing.T) {
	redisDB := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	g := NewRedisson(redisDB, WithWatchDogTimeout(time.Second))
	lock := g.GetLock("TestMutexRenew")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := lock.LockContext(ctx)
	if err != nil {
		panic(err)
	}

	time.Sleep(15 * time.Second)
	err = lock.Unlock()
	if err != nil {
		panic(err)
	}

}

// singleLockUnlockTest test single lock and unlock
func singleLockUnlockTest(times int32, variableName string, g *Redisson) error {
	lock := g.GetLock("plus_" + variableName)
	a := 0
	wg := sync.WaitGroup{}
	total := int32(0)
	for i := int32(0); i < times; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			err := lock.LockContext(ctx)
			if err != nil {
				return
			}
			a++
			err = lock.Unlock()
			if err != nil {
				panic("unlock failed")
			}
			atomic.AddInt32(&total, 1)
		}()
	}
	wg.Wait()
	if int32(a) != total {
		return fmt.Errorf("lock lock and unlock test failed, %s shoule equal %d,but equal %d", variableName, total, a)
	}
	return nil
}

// TestMutex_LockUnlock test lock lock and unlock
func TestMutex_LockUnlock(t *testing.T) {
	testCase := []int32{1, 10, 100, 200, 300, 330}
	for _, v := range testCase {
		if err := singleLockUnlockTest(v, "variable_1", GetRedisson()); err != nil {
			t.Fatalf("err=%v", err)
		}
	}
}

// TestMultiMutex test multi lock
func TestMultiMutex(t *testing.T) {
	testCases := []int32{1, 10, 100, 200}
	id := 0
	getVariableId := func() int {
		id++
		return id
	}
	for _, v := range testCases {
		wg := sync.WaitGroup{}
		numOfFailures := int32(0)
		for i := int32(0); i < v; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := singleLockUnlockTest(10, fmt.Sprintf("variable_%d", getVariableId()), GetRedisson())
				if err != nil {
					atomic.AddInt32(&numOfFailures, 1)
					return
				}
			}()
			wg.Wait()
		}
		if numOfFailures != 0 {
			t.Fatalf("multi lock test failed, numOfFailures should equal 0,but equal %d", numOfFailures)
		}
	}
}

// TestLockFairness test lock fairness
func TestLockFairness(t *testing.T) {
	g := GetRedisson()
	mu := g.GetLock("TestLockFairness")
	stop := make(chan bool)
	defer close(stop)
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			err := mu.LockContext(ctx)
			cancel()
			if err != nil {
				panic(err)
			}
			time.Sleep(100 * time.Microsecond)
			err = mu.Unlock()
			if err != nil {
				panic(err)
			}
			select {
			case <-stop:
				return
			default:
			}
		}
	}()
	done := make(chan bool, 1)
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Microsecond)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			err := mu.LockContext(ctx)
			cancel()
			if err != nil {
				panic(err)
			}
			err = mu.Unlock()
			if err != nil {
				panic(err)
			}
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("can't acquire Mutex in 10 seconds")
	}
}

// benchmarkLock benchmark lock
func benchmarkLock(b *testing.B, slack, work bool) {
	mu := GetRedisson().GetLock("benchmarkLock")
	if slack {
		b.SetParallelism(10)
	}
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := mu.LockContext(ctx)
			cancel()
			if err != nil {
				panic(err)
			}
			err = mu.Unlock()
			if err != nil {
				panic(err)
			}
			if work {
				for i := 0; i < 100; i++ {
					foo *= 2
					foo /= 2
				}
			}
		}
		_ = foo
	})
}

func BenchmarkLock(b *testing.B) {
	benchmarkLock(b, false, false)
}

func BenchmarkLockSlack(b *testing.B) {
	benchmarkLock(b, true, false)
}

func BenchmarkLockWork(b *testing.B) {
	benchmarkLock(b, false, true)
}

func BenchmarkLockWorkSlack(b *testing.B) {
	benchmarkLock(b, true, true)
}

func HammerLock(m Lock, loops int, cdone chan bool) {
	for i := 0; i < loops; i++ {
		if i%3 == 0 {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if m.LockContext(ctx) == nil {
				err := m.Unlock()
				if err != nil {
					panic(err)
				}
			}
			cancel()
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := m.LockContext(ctx)
		cancel()
		if err != nil {
			panic(err)
		}
		err = m.Unlock()
		if err != nil {
			panic(err)
		}
	}
	cdone <- true
}

func TestLock(t *testing.T) {
	if n := runtime.SetMutexProfileFraction(1); n != 0 {
		t.Logf("got mutexrate %d expected 0", n)
	}
	defer runtime.SetMutexProfileFraction(0)

	m := GetRedisson().GetLock("TestLock")

	c := make(chan bool)
	for i := 0; i < 10; i++ {
		go HammerLock(m, 1000, c)
	}
	for i := 0; i < 10; i++ {
		<-c
	}
}

func TestUnlockWithoutLocking(t *testing.T) {
	lock := GetRedisson().GetLock("TestUnlockWithoutLocking")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := lock.LockContext(ctx)
	cancel()
	if err != nil {
		panic(err)
	}
	go func() {
		err := lock.Unlock()
		if err == nil {
			panic("it should not be nil")
		}
	}()
	time.Sleep(1 * time.Second)
	err = lock.Unlock()
	if err != nil {
		panic(err)
	}
}

func TestRedisConnFailure(t *testing.T) {
	redisDB := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	g := NewRedisson(redisDB)
	_ = redisDB.Close()
	lock := g.GetLock("TestRedisConnFailure")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := lock.LockContext(ctx)
	cancel()
	if err == nil {
		panic("it should not be nil")
	}
	err = lock.Unlock()
	if err == nil {
		panic("it should not be nil")
	}
}

func TestLockBackground(t *testing.T) {
	l := GetRedisson().GetLock("TestLockBackground")
	a := 0
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		innerWg := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				err := l.Lock()
				if err != nil {
					panic(err)
				}
				a++
				err = l.Unlock()
				if err != nil {
					panic(err)
				}
			}()
		}
		innerWg.Wait()
	}()
	wg.Wait()
	if a != 100 {
		panic(a)
	}
}
