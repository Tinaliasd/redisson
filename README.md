# Redisson-Go: 分布式锁和工具库
## 项目背景
在分布式系统中，锁和限流器是常见的工具，用于保证数据一致性和系统稳定性。Redis 是一款高性能的内存数据库，支持分布式部署和多种数据结构，适合作为分布式锁和限流器的后端存储。

然而经过调研 Go 的 Redis 分布式锁的封装，大部分为几百行代码的简单包装，不具备看门狗续期等功能，且提供分布式锁类型少和缺少如限流器，延时队列，布隆过滤器等开发常用数据结构

故参考 Redisson 封装最完善 Redis包，并为了减少代码开发量，目前选择 Go 的 star量最高的 redis-go 进行封装，实现了 Redisson-Go，提供了分布式锁、原子变量、布隆过滤器、限流器等功能，方便开发者在 Go 项目中使用 Redis 进行分布式开发。
## 功能概述
Redisson-Go 是一个基于 Redis 的分布式工具库，灵感来源于 Java 的 Redisson 实现。它提供以下功能：
- **分布式锁**：支持可重入锁、互斥锁、读写锁等，提供看门狗续期机制和 Pub/Sub 通知唤醒机制。
- **原子变量**：支持 `AtomicLong` 和 `AtomicDouble`。
- **布隆过滤器**：高效的集合判断工具。
- **BitSet**：位操作支持。
- **限流器**：令牌桶算法实现。
- **延时队列**：正在加快速度进行开发准备上线。
## 安装
```bash
go get github.com/Tinaliasd/redisson
```

## 快速开始

### 初始化客户端
在使用 Redisson-Go 之前，需要先初始化 Redis 客户端和 Redisson 实例：
```go
package main

import (
    "github.com/redis/go-redis/v9"
    "github.com/Tinaliasd/redisson"
    "time"
)

func main() {
    // 创建 Redis 客户端
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // 初始化 Redisson 客户端
    r := redisson.NewRedisson(redisClient, redisson.WithWatchDogTimeout(30*time.Second))

    // 示例：获取分布式锁
    lock := r.GetLock("myLock")
    if err := lock.Lock(); err != nil {
        panic(err)
    }
    defer lock.Unlock()
}
```

## 功能详解

### **分布式锁**
分布式锁支持可重入锁、互斥锁以及读写锁。

#### 使用示例
```go
lock := r.GetLock("resourceKey")
if err := lock.Lock(); err != nil {
    panic("无法获取锁")
}
// 执行临界区代码...
lock.Unlock()
```

#### 接口说明
- `GetLock(key string)`: 获取可重入锁。
- `GetMutex(key string)`: 获取不可重入的互斥锁。
- `GetReadWriteLock(key string)`: 获取读写锁。

读写锁操作示例：
```go
rwLock := r.GetReadWriteLock("rwResource")
readLock := rwLock.ReadLock()
writeLock := rwLock.WriteLock()

// 获取读锁
readLock.Lock()
defer readLock.Unlock()

// 获取写锁
writeLock.Lock()
defer writeLock.Unlock()
```

---

### **原子变量**
支持整型和浮点型原子变量，适用于计数器等场景。

#### 使用示例
```go
atomicLong := r.GetAtomicLong("counter")
atomicLong.Set(100)
value, _ := atomicLong.AddAndGet(10)
fmt.Println(value) // 输出 110
```

#### 接口说明
- `GetAtomicLong(key string)`: 获取整型原子变量。
- `GetAtomicDouble(key string)`: 获取浮点型原子变量。
- 常用方法：
    - `AddAndGet(delta int64/float64)`
    - `CompareAndSet(expect, update)`
    - `IncrementAndGet()`
    - `Set(value)`

---

### **布隆过滤器**
布隆过滤器是一种高效的集合判断工具，适合大规模数据场景。

#### 使用示例
```go
bloomFilter := redisson.GetBloomFilter[string](r, "myBloomFilter")
bloomFilter.TryInit(10000, 0.01) // 初始化，期望插入 10000 个元素，误判率 1%

bloomFilter.Add("hello")
exists := bloomFilter.Contains("hello")
fmt.Println(exists) // 输出 true
```

#### 接口说明
- `TryInit(expectedInsertions int64, falseProbability float64)`: 初始化过滤器。
- `Add(obj T)`: 添加元素。
- `Contains(obj T)`: 检查元素是否存在。

---

### **BitSet**
提供基于 Redis 的位操作支持。

#### 使用示例
```go
bitSet := r.GetBitSet("myBitSet")
bitSet.SetByte(0, 1)
value, _ := bitSet.GetByte(0)
fmt.Println(value) // 输出 1
```

#### 接口说明
- 位操作方法：
    - `GetByte(offset int64)`
    - `SetByte(offset int64, value byte)`
    - 支持其他类型如 `int16`, `int32`, `int64` 的类似操作。

---

### **限流器**
基于令牌桶算法实现的分布式限流器。

#### 使用示例
```go
rateLimiter := r.GetRateLimiter("apiLimiter")
rateLimiter.TrySetRate(redisson.RateTypeOVERALL, 10, 1, redisson.Seconds)

if ok, _ := rateLimiter.TryAcquire(); ok {
    fmt.Println("成功获取许可")
} else {
    fmt.Println("限流中...")
}
```

#### 接口说明
- `TrySetRate(mode RateType, rate int64, interval int64, unit RateIntervalUnit)`: 设置限流配置。
- `TryAcquire()`: 尝试获取一个许可。
- `Acquire()`: 阻塞直到获取许可。
- `AvailablePermits()`: 返回当前可用许可数量。

---

## 配置选项

Redisson 支持通过选项函数进行配置：
- **`WithWatchDogTimeout(duration time.Duration)`**: 配置看门狗超时时间（默认 30 秒）。

---

## 注意事项

1. 确保 Redis 服务稳定运行，避免因网络问题导致锁超时或丢失。
2. 对于高并发场景，建议合理设置看门狗超时时间和限流参数。

---

## 开源协议

本项目遵循 MIT 协议，欢迎贡献代码或提出建议！

Citations:
[1] https://ppl-ai-file-upload.s3.amazonaws.com/web/direct-files/35020974/5be51180-4058-47a2-a147-0c815c469f54/paste.txt