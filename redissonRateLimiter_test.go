package redisson

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestRR(t *testing.T) {
	fmt.Println("Starting TestRR...")

	// 假设不同客户端用不同 clientID 区分
	red := GetRedisson()

	// 获取一个限流器
	rl := red.GetRateLimiter("myRateLimiter")

	// 仅当尚未设置过时才初始化
	ok, err := rl.TrySetRate(RateTypeOVERALL, 10, 1, Seconds)
	if err != nil {
		panic(err)
	}
	if !ok {
		fmt.Println("TrySetRate returned false, might already be set.")
	} else {
		fmt.Println("TrySetRate succeeded: RateLimiter initialized successfully.")
	}

	// 更新速率配置
	err = rl.SetRate(RateTypeOVERALL, 10, 1, Seconds)
	if err != nil {
		fmt.Println("SetRate failed:", err)
	} else {
		fmt.Println("SetRate succeeded: RateLimiter configuration updated.")
	}

	// 获取配置
	fmt.Println("Fetching current RateLimiter configuration...")
	config, err := rl.GetConfig()
	if err != nil {
		fmt.Println("GetConfig failed:", err)
	} else {
		fmt.Printf("Current RateLimiter config: %+v\n", config)
	}

	// 尝试获取 1 个许可（非阻塞）
	fmt.Println("Attempting to acquire 1 permit...")
	got, err := rl.TryAcquire()
	if err != nil {
		fmt.Println("TryAcquire failed:", err)
	} else {
		fmt.Printf("TryAcquire => success: %v\n", got)
	}

	// 需要阻塞获取 5 个许可
	go func() {
		fmt.Println("Acquiring 5 permits (blocking)...")
		start := time.Now()
		err := rl.AcquirePermits(5)
		elapsed := time.Since(start)
		if err != nil {
			fmt.Printf("AcquirePermits error after %v: %v\n", elapsed, err)
		} else {
			fmt.Printf("AcquirePermits(5) done after %v.\n", elapsed)
		}
	}()

	// 演示可用许可
	for i := 0; i < 3; i++ {
		time.Sleep(1500 * time.Millisecond)
		available, err := rl.AvailablePermits()
		if err != nil {
			fmt.Printf("[%d] AvailablePermits failed: %v\n", i+1, err)
		} else {
			fmt.Printf("[%d] Available permits => %d (at %v)\n", i+1, available, time.Now().Format(time.RFC3339))
		}
	}

	time.Sleep(3 * time.Second)

	available, err := rl.AvailablePermits()
	if err != nil {
		fmt.Println("Final AvailablePermits check failed:", err)
	} else {
		fmt.Printf("Final available permits => %d (at %v)\n", available, time.Now().Format(time.RFC3339))
	}

	fmt.Println("TestRR completed.")
}

func TestRateTypeMarshal(t *testing.T) {
	original := RateTypeOVERALL
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	var unmarshaled RateType
	err = unmarshaled.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if original != unmarshaled {
		t.Fatalf("Expected %v, got %v", original, unmarshaled)
	}
}

func TestSingleClientMultiThread(t *testing.T) {
	red := GetRedisson()
	rl := red.GetRateLimiter("singleClientLimiter")
	rl.Expire(10 * time.Second)
	// 设置限流配置，每秒2个许可
	if err := rl.SetRate(RateTypeOVERALL, 2, 1, Seconds); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make(chan time.Time, 4)
	start := time.Now()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Acquire(); err != nil {
				t.Errorf("Acquire failed: %v", err)
			}
			results <- time.Now()
		}()
	}

	wg.Wait()
	close(results)

	var times []time.Duration
	for t := range results {
		times = append(times, t.Sub(start))
	}

	// 排序
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	// 检查前两个请求是否在第0-1秒内完成，后两个在1-2秒内完成
	if times[0] < 1*time.Second && times[1] < 1*time.Second &&
		times[2] >= 1*time.Second && times[2] < 2*time.Second &&
		times[3] >= 1*time.Second && times[3] < 2*time.Second {
		t.Log("限流器工作正常")
	} else {
		t.Errorf("请求时间不符合预期: %v", times)
	}
}

//func TestRateLimiter_InsufficientTokens(t *testing.T) {
//	fmt.Println("Starting TestRateLimiter_InsufficientTokens...")
//
//	// 初始化 Redis 客户端和限流器
//	red := GetRedisson()
//	rl := red.GetRateLimiter("myRateLimiter")
//
//	// 设置限流器配置：每秒最多生成 5 个令牌
//	err := rl.SetRate(RateTypeOVERALL, 5, 1, Seconds)
//	if err != nil {
//		t.Fatalf("Failed to set rate: %v", err)
//	}
//	fmt.Println("RateLimiter configured: 5 tokens per second.")
//
//	for {
//		available, _ := rl.AvailablePermits()
//
//		fmt.Printf("Available permits before consumption: %d\n", available)
//
//		if available == 0 {
//			fmt.Println("All tokens consumed.")
//			break
//		}
//
//		success, err := rl.TryAcquire()
//		if err != nil {
//			t.Fatalf("Error in TryAcquire: %v\n", err)
//		}
//		//time.Sleep(5 * time.Second)
//		if !success {
//			t.Fatalf("Failed to acquire permit even though permits are available.")
//		}
//	}
//	fmt.Println("All tokens consumed. Available permits => 0")
//
//	// 启动多个并发请求尝试获取许可
//	var wg sync.WaitGroup
//	numRequests := 10 // 模拟 10 个并发请求
//	results := make([]bool, numRequests)
//
//	for i := 0; i < numRequests; i++ {
//		wg.Add(1)
//		go func(idx int) {
//			defer wg.Done()
//			fmt.Printf("Request #%d attempting to acquire a permit...\n", idx+1)
//			success, err := rl.TryAcquireWithTimeout(2 * time.Second) // 每个请求等待最多 2 秒
//			if err != nil {
//				fmt.Printf("Request #%d failed with error: %v\n", idx+1, err)
//				results[idx] = false
//			} else {
//				results[idx] = success
//				if success {
//					fmt.Printf("Request #%d successfully acquired a permit.\n", idx+1)
//				} else {
//					fmt.Printf("Request #%d timed out waiting for a permit.\n", idx+1)
//				}
//			}
//
//			// 打印当前可用许可数量
//			available, _ := rl.AvailablePermits()
//			fmt.Printf("Request #%d current available permits: %d\n", idx+1, available)
//		}(i)
//	}
//
//	// 等待所有请求完成
//	wg.Wait()
//
//	// 打印测试结果
//	successCount := 0
//	for i, result := range results {
//		if result {
//			successCount++
//		}
//		fmt.Printf("Request #%d result: %v\n", i+1, result)
//	}
//	fmt.Printf("Total successful requests: %d/%d\n", successCount, numRequests)
//
//	// 最终打印限流器状态
//	finalAvailablePermits, _ := rl.AvailablePermits()
//	fmt.Printf("Final available permits: %d\n", finalAvailablePermits)
//
//	fmt.Println("TestRateLimiter_InsufficientTokens completed.")
//}

func printRedisState(client *redis.Client) {
	ctx := context.Background()
	keys, err := client.Keys(ctx, "*").Result()
	if err != nil {
		log.Printf("Error retrieving keys: %v", err)
		return
	}
	for _, key := range keys {
		kind, err := client.Type(ctx, key).Result()
		if err != nil {
			log.Printf("Error getting type for key %s: %v", key, err)
			continue
		}
		log.Printf("Key: %s, Type: %s", key, kind)

		switch kind {
		case "string":
			val, err := client.Get(ctx, key).Result()
			if err != nil {
				log.Printf("Error getting value for key %s: %v", key, err)
				continue
			}
			log.Printf("Value: %s", val)
		case "hash":
			val, err := client.HGetAll(ctx, key).Result()
			if err != nil {
				log.Printf("Error getting hash for key %s: %v", key, err)
				continue
			}
			for field, value := range val {
				log.Printf("Field: %s, Value: %s", field, value)
			}
		case "zset":
			members, err := client.ZRangeWithScores(ctx, key, 0, -1).Result()
			if err != nil {
				log.Printf("Error getting zset for key %s: %v", key, err)
				continue
			}
			for _, member := range members {
				log.Printf("Member: %s, Score: %f", member.Member, member.Score)
			}
		default:
			log.Printf("Unknown type for key %s: %s", key, kind)
		}

		// Get TTL
		ttl, err := client.TTL(ctx, key).Result()
		if err != nil {
			log.Printf("Error getting TTL for key %s: %v", key, err)
			continue
		}
		log.Printf("TTL: %v", ttl)
	}
}

func TestRateLimiter_InsufficientTokens(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("Starting TestRateLimiter_InsufficientTokens...")

	// Initialize Redisson client and rate limiter
	red := GetRedisson()
	rl := red.GetRateLimiter("myRateLimiter")

	// Create Redis client for printing
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		DB:       0,
		Password: "", // if needed
	})
	defer redisClient.Close()

	// Create a channel to signal the printing goroutine to stop
	done := make(chan struct{})

	// Launch the printing goroutine
	go func() {
		printRedisState(redisClient) // Initial print
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				printRedisState(redisClient)
			}
		}
	}()

	// Set rate limiter configuration
	err := rl.SetRate(RateTypeOVERALL, 5, 1, Seconds)
	if err != nil {
		t.Fatalf("Failed to set rate: %v", err)
	}
	log.Printf("RateLimiter configured: 5 tokens per second.")

	// Consume all available permits
	for {
		available, _ := rl.AvailablePermits()
		log.Printf("Available permits before consumption: %d", available)
		if available == 0 {
			log.Printf("All tokens consumed.")
			break
		}
		success, err := rl.TryAcquire()
		if err != nil {
			t.Fatalf("Error in TryAcquire: %v", err)
		}
		time.Sleep(4000 * time.Millisecond)
		if !success {
			t.Fatalf("Failed to acquire permit even though permits are available.")
		}
	}
	log.Printf("All tokens consumed. Available permits => 0")

	// Simulate concurrent requests
	var wg sync.WaitGroup
	numRequests := 10
	results := make([]bool, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			log.Printf("Request #%d attempting to acquire a permit...", idx+1)
			success, err := rl.TryAcquireWithTimeout(2 * time.Second)
			if err != nil {
				log.Printf("Request #%d failed with error: %v", idx+1, err)
				results[idx] = false
			} else {
				results[idx] = success
				if success {
					log.Printf("Request #%d successfully acquired a permit.", idx+1)
				} else {
					log.Printf("Request #%d timed out waiting for a permit.", idx+1)
				}
			}
			available, _ := rl.AvailablePermits()
			log.Printf("Request #%d current available permits: %d", idx+1, available)
		}(i)
	}

	wg.Wait()

	// Print test results
	successCount := 0
	for i, result := range results {
		if result {
			successCount++
		}
		log.Printf("Request #%d result: %v", i+1, result)
	}
	log.Printf("Total successful requests: %d/%d", successCount, numRequests)

	// Final print of available permits
	finalAvailablePermits, _ := rl.AvailablePermits()
	log.Printf("Final available permits: %d", finalAvailablePermits)

	// Signal the printing goroutine to stop
	close(done)

	log.Printf("TestRateLimiter_InsufficientTokens completed.")
}

func TestRateLimiter_TokenReplenishment(t *testing.T) {
	// Create Redis client for printing
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		DB:       0,
		Password: "", // if needed
	})
	defer redisClient.Close()

	// Create a channel to signal the printing goroutine to stop
	done := make(chan struct{})

	// Launch the printing goroutine
	go func() {
		printRedisState(redisClient) // Initial print
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				printRedisState(redisClient)
			}
		}
	}()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("Starting TestRateLimiter_TokenReplenishment...")

	red := GetRedisson()
	rl := red.GetRateLimiter("myRateLimiter")

	// 设置速率：每秒5个令牌
	err := rl.SetRate(RateTypeOVERALL, 5, 1, Seconds)
	if err != nil {
		t.Fatalf("Failed to set rate: %v", err)
	}
	log.Printf("RateLimiter configured: 5 tokens per second")

	// 先消耗所有令牌
	for {
		available, _ := rl.AvailablePermits()
		if available == 0 {
			break
		}
		success, err := rl.TryAcquire()
		if err != nil {
			t.Fatalf("Failed to acquire initial permits: %v", err)
		}
		if !success {
			break
		}
	}
	log.Printf("All initial tokens consumed")

	// 等待1秒，让令牌补充
	time.Sleep(2 * time.Second)

	// 并发测试令牌获取
	var wg sync.WaitGroup
	numRequests := 10
	results := make([]bool, numRequests)

	// 记录开始时间
	startTime := time.Now()

	// 启动并发请求
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// 随机等待0-500ms，模拟真实请求
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			log.Printf("Request #%d attempting to acquire permit at %v",
				idx+1, time.Since(startTime))

			success, err := rl.TryAcquire()
			if err != nil {
				log.Printf("Request #%d failed with error: %v", idx+1, err)
				results[idx] = false
				return
			}

			results[idx] = success
			available, _ := rl.AvailablePermits()
			log.Printf("Request #%d: success=%v, available=%d, time=%v",
				idx+1, success, available, time.Since(startTime))
		}(i)
	}

	wg.Wait()

	// 统计结果
	successCount := 0
	for i, result := range results {
		if result {
			successCount++
		}
		log.Printf("Request #%d result: %v", i+1, result)
	}

	// 验证结果
	expectedSuccess := 5 // 应该有5个请求成功（因为每秒补充5个令牌）
	if successCount != expectedSuccess {
		t.Errorf("Expected %d successful requests, got %d", expectedSuccess, successCount)
	}

	close(done)
	log.Printf("TestRateLimiter_TokenReplenishment completed")
}
func TestRateLimiter_TokenReplenishmentProcess(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("Starting TestRateLimiter_TokenReplenishmentProcess...")

	red := GetRedisson()
	rl := red.GetRateLimiter("myRateLimiter")

	// 监控 Redis 状态
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})
	defer redisClient.Close()

	// 设置速率：每秒5个令牌
	err := rl.SetRate(RateTypeOVERALL, 5, 1, Seconds)
	if err != nil {
		t.Fatalf("Failed to set rate: %v", err)
	}
	log.Printf("RateLimiter configured: 5 tokens per second")

	// 只消耗2个令牌，保持低于最大容量
	for i := 0; i < 2; i++ {
		success, err := rl.TryAcquire()
		if err != nil {
			t.Fatalf("Failed to acquire initial permits: %v", err)
		}
		if !success {
			t.Fatalf("Failed to acquire permit %d", i+1)
		}
		available, _ := rl.AvailablePermits()
		log.Printf("Consumed token %d, available permits: %d", i+1, available)
	}

	// 观察5秒内的令牌补充过程
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		available, _ := rl.AvailablePermits()

		// 打印当前 Redis 状态
		printRedisState(redisClient)

		log.Printf("After %d second(s), available permits: %d", i+1, available)
	}
}

func TestRateLimiter_TokenReplenishment1(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("Starting TestRateLimiter_TokenReplenishment...")

	red := GetRedisson()
	rl := red.GetRateLimiter("myRateLimiter")

	// 设置速率：每秒5个令牌
	err := rl.SetRate(RateTypeOVERALL, 5, 1, Seconds)
	if err != nil {
		t.Fatalf("Failed to set rate: %v", err)
	}
	log.Printf("RateLimiter configured: 5 tokens per second")

	// 打印初始状态
	printRedisState(red.client)

	// 消耗3个令牌
	for i := 0; i < 3; i++ {
		_, err := rl.TryAcquire()
		if err != nil {
			t.Fatalf("Failed to acquire permit: %v", err)
		}
		available, _ := rl.AvailablePermits()
		log.Printf("Consumed token %d, available permits: %d", i+1, available)
		time.Sleep(100 * time.Millisecond)
	}

	// 等待500ms观察补充情况
	time.Sleep(500 * time.Millisecond)
	available, _ := rl.AvailablePermits()
	log.Printf("After 500ms, available permits: %d", available)

	// 再次消耗令牌测试补充效果
	for i := 0; i < 3; i++ {
		success, err := rl.TryAcquire()
		if err != nil {
			t.Fatalf("Failed to acquire permit: %v", err)
		}
		if !success {
			t.Errorf("Failed to acquire permit after replenishment")
		}
		available, _ := rl.AvailablePermits()
		log.Printf("After replenishment - Consumed token %d, available permits: %d", i+1, available)
		time.Sleep(2500 * time.Millisecond)
	}

	// 最终验证
	time.Sleep(1 * time.Second)
	finalAvailable, _ := rl.AvailablePermits()
	time.Sleep(25 * time.Second)
	log.Printf("Final available permits: %d", finalAvailable)
	if finalAvailable != 5 {
		t.Errorf("Expected 5 permits after replenishment, got %d", finalAvailable)
	}
}
