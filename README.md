# Redisson-Go: 分布式锁和工具库

## 功能概述
Redisson-Go 是一个基于 Redis 的分布式工具库，灵感来源于 Java 的 Redisson 实现。它提供以下功能：
- **分布式锁**：支持可重入锁、互斥锁、读写锁。
- **原子变量**：支持 `AtomicLong` 和 `AtomicDouble`。
- **布隆过滤器**：高效的集合判断工具。
- **BitSet**：位操作支持。
- **限流器**：令牌桶算法实现。

## 安装
```bash
go get github.com/your/repo/redisson
```

## 快速开始

### 初始化客户端
在使用 Redisson-Go 之前，需要先初始化 Redis 客户端和 Redisson 实例：
```go
package main

import (
    "github.com/redis/go-redis/v9"
    "github.com/your/repo/redisson"
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