package redisson

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestMutexRenew test mutex renew
func TestMutexRenew(t *testing.T) {
	g := GetRedisson()
	mutex := g.GetMutex("TestMutexRenew")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := mutex.LockContext(ctx)
	if err != nil {
		panic(err)
	}

	time.Sleep(35 * time.Second)
	err = mutex.Unlock()
	if err != nil {
		panic(err)
	}
}

// testMutexLock test mutex lock unlock
func testMutexLock(times int) {
	l := GetRedisson().GetMutex("testMutexLock")
	a := 0
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		innerWg := sync.WaitGroup{}
		for i := 0; i < times; i++ {
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				err := l.LockContext(ctx)
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
	if a != times {
		panic(a)
	}
}

func TestMutexLock(t *testing.T) {
	for _, v := range []int{1, 10, 100, 200, 300, 400} {
		testMutexLock(v)
	}
}

func HammerMutex(m Lock, loops int, cdone chan bool) {
	for i := 0; i < loops; i++ {
		if i%3 == 0 {
			if m.Lock() == nil {
				err := m.Unlock()
				if err != nil {
					fmt.Println("Unlock failed with mutex unlocked")
				}
			}
			continue
		}
		err := m.Lock()
		if err != nil {
			fmt.Println("Lock failed with mutex unlocked")
		}
		err = m.Unlock()
		if err != nil {
			fmt.Println("Unlock failed with mutex unlocked")
		}

	}
	cdone <- true
}

func TestMutex(t *testing.T) {
	if n := runtime.SetMutexProfileFraction(1); n != 0 {
		t.Logf("got mutexrate %d expected 0", n)
	}
	defer runtime.SetMutexProfileFraction(0)

	m := GetRedisson().GetMutex("TestMutex1")

	err := m.Lock()
	if err != nil {
		t.Fatalf("Lock failed with mutex unlocked")
	}
	ctx, cancle := context.WithTimeout(context.Background(), time.Second)
	if m.LockContext(ctx) == nil {
		cancle()
		t.Fatalf("TryLock succeeded with mutex locked")
	}
	cancle()
	err = m.Unlock()
	if err != nil {
		t.Fatalf("Unlock failed with mutex unlocked,err=%v", err)
	}
	ctx, cancle = context.WithTimeout(context.Background(), time.Second)
	if m.LockContext(ctx) != nil {
		cancle()
		t.Fatalf("TryLock failed with mutex unlocked")
	}
	cancle()
	err = m.Unlock()
	if err != nil {
		t.Fatalf("Unlock failed with mutex unlocked")
	}

	c := make(chan bool)
	for i := 0; i < 10; i++ {
		go HammerMutex(m, 1000, c)
	}
	for i := 0; i < 10; i++ {
		<-c
	}
}

func TestMutexFairness(t *testing.T) {
	mu := GetRedisson().GetMutex("testMutexFairness1")
	stop := make(chan bool)
	defer close(stop)
	go func() {
		for {
			err := mu.Lock()
			if err != nil {
				panic(fmt.Errorf("can't acquire Mutex: %v", err))
			}
			time.Sleep(100 * time.Microsecond)
			err = mu.Unlock()
			if err != nil {
				panic(fmt.Errorf("can't release Mutex: %v", err))
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
			err := mu.Lock()
			if err != nil {
				panic(fmt.Errorf("can't acquire Mutex: %v", err))
			}
			err = mu.Unlock()
			if err != nil {
				panic(fmt.Errorf("can't release Mutex: %v", err))
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

func BenchmarkMutexUncontended(b *testing.B) {
	type PaddedMutex struct {
		mu  Lock
		pad [128]uint8
	}

	b.RunParallel(func(pb *testing.PB) {
		var mu PaddedMutex
		mu.mu = GetRedisson().GetMutex("benchmarkMutexUncontended")
		for pb.Next() {
			err := mu.mu.Lock()
			if err != nil {
				b.Fatalf("Lock failed: %v", err)

			}
			err = mu.mu.Unlock()
			if err != nil {
				b.Fatalf("Unlock failed: %v", err)

			}
		}
	})
}

func benchmarkMutex(b *testing.B, slack, work bool) {
	mu := GetRedisson().GetMutex("benchmarkMutex11")
	if slack {
		b.SetParallelism(10)
	}
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			err := mu.Lock()
			if err != nil {
				b.Fatalf("Lock failed: %v", err)
			}
			err = mu.Unlock()
			if err != nil {
				b.Fatalf("Unlock failed: %v", err)
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

func BenchmarkMutex(b *testing.B) {
	benchmarkMutex(b, false, false)
}

func BenchmarkMutexSlack(b *testing.B) {
	benchmarkMutex(b, true, false)
}

func BenchmarkMutexWork(b *testing.B) {
	benchmarkMutex(b, false, true)
}

func BenchmarkMutexWorkSlack(b *testing.B) {
	benchmarkMutex(b, true, true)
}

func BenchmarkMutexNoSpin(b *testing.B) {
	// This benchmark models a situation where spinning in the mutex should be
	// non-profitable and allows to confirm that spinning does not do harm.
	// To achieve this we create excess of goroutines most of which do local work.
	// These goroutines yield during local work, so that switching from
	// a blocked goroutine to other goroutines is profitable.
	// As a matter of fact, this benchmark still triggers some spinning in the mutex.
	m := GetRedisson().GetMutex("BenchmarkMutexNoSpin")
	var acc0, acc1 uint64
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		c := make(chan bool)
		var data [4 << 10]uint64
		for i := 0; pb.Next(); i++ {
			if i%4 == 0 {
				err := m.Lock()
				if err != nil {
					b.Fatalf("Lock failed: %v", err)

				}
				acc0 -= 100
				acc1 += 100
				err = m.Unlock()
				if err != nil {
					b.Fatalf("Unlock failed: %v", err)

				}
			} else {
				for i := 0; i < len(data); i += 4 {
					data[i]++
				}
				// Elaborate way to say runtime.Gosched
				// that does not put the goroutine onto global runq.
				go func() {
					c <- true
				}()
				<-c
			}
		}
	})
}

func BenchmarkMutexSpin(b *testing.B) {
	// This benchmark models a situation where spinning in the mutex should be
	// profitable. To achieve this we create a goroutine per-proc.
	// These goroutines access considerable amount of local data so that
	// unnecessary rescheduling is penalized by cache misses.
	m := GetRedisson().GetMutex("BenchmarkMutexSpin")
	var acc0, acc1 uint64
	b.RunParallel(func(pb *testing.PB) {
		var data [16 << 10]uint64
		for i := 0; pb.Next(); i++ {
			err := m.Lock()
			if err != nil {
				b.Fatalf("Lock failed: %v", err)
			}
			acc0 -= 100
			acc1 += 100
			err = m.Unlock()
			if err != nil {
				b.Fatalf("Unlock failed: %v", err)
			}
			for i := 0; i < len(data); i += 4 {
				data[i]++
			}
		}
	})
}

func TestMutexRedisConnFailure(t *testing.T) {
	redisDB := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	g := NewRedisson(redisDB)
	_ = redisDB.Close()
	lock := g.GetMutex("TestMutexRedisConnFailure")
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

func TestMutexUnlockFailed(t *testing.T) {
	lock := GetRedisson().GetMutex("TestMutexUnlockFailed")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := lock.LockContext(ctx)
		cancel()
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(time.Second)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := lock.UnlockContext(ctx)
		cancel()
		if err == nil {
			panic("it should not be nil")
		}
	}()
	time.Sleep(2 * time.Second)
}
