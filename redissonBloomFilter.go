package redisson

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"math"
)

// RBloomFilter represents a Redis-backed Bloom filter
type RBloomFilter[T any] interface {
	// Add adds an element to the Bloom filter
	// Returns true if element was added successfully
	// Returns false if element is already present
	Add(object T) bool

	// Contains checks if an element is present in the Bloom filter
	// Returns true if element is present
	// Returns false if element is not present
	Contains(object T) bool

	// TryInit initializes Bloom filter parameters (size and hashIterations)
	// calculated from expectedInsertions and falseProbability
	// Stores config to Redis server
	// Returns true if Bloom filter was initialized
	// Returns false if Bloom filter was already initialized
	TryInit(expectedInsertions int64, falseProbability float64) bool

	// GetExpectedInsertions returns expected amount of insertions per element
	// Calculated during bloom filter initialization
	GetExpectedInsertions() int64

	// GetFalseProbability returns false probability of element presence
	// Calculated during bloom filter initialization
	GetFalseProbability() float64

	// GetSize returns number of bits in Redis memory required by this instance
	GetSize() int64

	// GetHashIterations returns hash iterations amount used per element
	// Calculated during bloom filter initialization
	GetHashIterations() int

	// Count calculates probabilistic number of elements already added to Bloom filter
	Count() int64

	// Embedded interface for expiration functionality
	RExpirable
}

// RedissonBloomFilter 实现 RBloomFilter 接口
type RedissonBloomFilter[T any] struct {
	*RedissonExpirable
	key            string
	size           int64  // 布隆过滤器的位数组大小
	hashIterations int    // hash函数的迭代次数
	configName     string // 配置名称，用于存储布隆过滤器的配置

}

// NewRedissonBloomFilter 构造函数
func NewRedissonBloomFilter[T any](redisson *Redisson, key string) *RedissonBloomFilter[T] {
	configName := suffixName(key, "config")
	return &RedissonBloomFilter[T]{
		RedissonExpirable: newRedissonExpirable(key, redisson),
		key:               key,
		configName:        configName,
	}
}

// TryInit 初始化布隆过滤器
func (bf *RedissonBloomFilter[T]) TryInit(expectedInsertions int64, falseProbability float64) bool {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	// 检查是否已经初始化
	exists, err := bf.client.Exists(context.Background(), bf.configName).Result()
	if err != nil {
		fmt.Printf("Error checking Bloom filter config existence: %v\n", err)
		return false
	}
	if exists != 0 {
		// 已经初始化
		return false
	}

	// 计算布隆过滤器的大小和哈希迭代次数
	size, hashIterations := optimalBloomParameters(expectedInsertions, falseProbability)

	// 构建配置
	config := BloomConfig{
		ExpectedInsertions: expectedInsertions,
		FalseProbability:   falseProbability,
		Size:               size,
		HashIterations:     hashIterations,
	}

	// 将配置存储到 Redis
	configBytes, err := json.Marshal(config)
	if err != nil {
		fmt.Printf("Error marshaling Bloom filter config: %v\n", err)
		return false
	}

	// 使用事务确保原子性
	pipe := bf.client.TxPipeline()
	pipe.SetNX(context.Background(), bf.configName, configBytes, 0)
	_, err = pipe.Exec(context.Background())
	if err != nil {
		fmt.Printf("Error setting Bloom filter config: %v\n", err)
		return false
	}

	// 更新本地配置
	bf.size = size
	bf.hashIterations = hashIterations

	return true
}

// Add 添加元素到布隆过滤器
func (bf *RedissonBloomFilter[T]) Add(object T) bool {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	// 如果未初始化，尝试初始化
	if bf.size == 0 || bf.hashIterations == 0 {
		// 尝试读取配置
		err := bf.readConfig()
		if err != nil {
			fmt.Printf("Bloom filter not initialized: %v\n", err)
			return false
		}
	}

	// 计算哈希索引
	indexes, err := bf.getHashIndexes(object)
	if err != nil {
		fmt.Printf("Error hashing object: %v\n", err)
		return false
	}

	// 设置位
	var anySet bool
	for _, idx := range indexes {
		set, err := bf.SetBit(idx, true)
		if err != nil {
			fmt.Printf("Error setting bit at index %d: %v\n", idx, err)
			return false
		}
		if set {
			anySet = true
		}
	}

	return anySet
}

// Contains 检查元素是否在布隆过滤器中
func (bf *RedissonBloomFilter[T]) Contains(object T) bool {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	// 如果未初始化，尝试初始化
	if bf.size == 0 || bf.hashIterations == 0 {
		// 尝试读取配置
		err := bf.readConfig()
		if err != nil {
			fmt.Printf("Bloom filter not initialized: %v\n", err)
			return false
		}
	}

	// 计算哈希索引
	indexes, err := bf.getHashIndexes(object)
	if err != nil {
		fmt.Printf("Error hashing object: %v\n", err)
		return false
	}

	// 检查位
	for _, idx := range indexes {
		exists, err := bf.GetBit(idx)
		if err != nil {
			fmt.Printf("Error getting bit at index %d: %v\n", idx, err)
			return false
		}
		if !exists {
			return false
		}
	}

	return true
}

// GetExpectedInsertions 返回预期插入量
func (bf *RedissonBloomFilter[T]) GetExpectedInsertions() int64 {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	config, err := bf.getConfig()
	if err != nil {
		fmt.Printf("Error getting Bloom filter config: %v\n", err)
		return 0
	}
	return config.ExpectedInsertions
}

// GetFalseProbability 返回假阳性概率
func (bf *RedissonBloomFilter[T]) GetFalseProbability() float64 {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	config, err := bf.getConfig()
	if err != nil {
		fmt.Printf("Error getting Bloom filter config: %v\n", err)
		return 0.0
	}
	return config.FalseProbability
}

// GetSize 返回布隆过滤器的位数组大小
func (bf *RedissonBloomFilter[T]) GetSize() int64 {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	config, err := bf.getConfig()
	if err != nil {
		fmt.Printf("Error getting Bloom filter config: %v\n", err)
		return 0
	}
	return config.Size
}

// GetHashIterations 返回哈希迭代次数
func (bf *RedissonBloomFilter[T]) GetHashIterations() int {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	config, err := bf.getConfig()
	if err != nil {
		fmt.Printf("Error getting Bloom filter config: %v\n", err)
		return 0
	}
	return config.HashIterations
}

// Count 估算已经添加的元素数量
func (bf *RedissonBloomFilter[T]) Count() int64 {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	// 获取设置的位数
	count, err := bf.client.BitCount(context.Background(), bf.key, &redis.BitCount{
		Start: 0,
		End:   -1,
	}).Result()
	if err != nil {
		fmt.Printf("Error counting bits in Bloom filter: %v\n", err)
		return 0
	}

	// 估算插入数量
	// 使用公式: n = -(m / k) * ln(1 - X/m)
	m := float64(bf.size)
	k := float64(bf.hashIterations)
	X := float64(count)

	if X == 0 {
		return 0
	}

	n := -(m / k) * math.Log(1-X/m)
	return int64(n)
}

// Helper Structures and Functions

// BloomConfig 存储布隆过滤器的配置
type BloomConfig struct {
	ExpectedInsertions int64   `json:"expectedInsertions"`
	FalseProbability   float64 `json:"falseProbability"`
	Size               int64   `json:"size"`
	HashIterations     int     `json:"hashIterations"`
}

// readConfig 从 Redis 中读取布隆过滤器的配置
func (bf *RedissonBloomFilter[T]) readConfig() error {
	data, err := bf.client.Get(context.Background(), bf.configName).Bytes()
	if err != nil {
		return fmt.Errorf("failed to get Bloom filter config: %v", err)
	}

	var config BloomConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Bloom filter config: %v", err)
	}

	bf.size = config.Size
	bf.hashIterations = config.HashIterations
	return nil
}

// getConfig 获取布隆过滤器的配置
func (bf *RedissonBloomFilter[T]) getConfig() (*BloomConfig, error) {
	data, err := bf.client.Get(context.Background(), bf.configName).Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get Bloom filter config: %v", err)
	}

	var config BloomConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Bloom filter config: %v", err)
	}

	return &config, nil
}

// getHashIndexes 计算元素的哈希索引
func (bf *RedissonBloomFilter[T]) getHashIndexes(object T) ([]int64, error) {
	// 序列化对象为 JSON
	objBytes, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %v", err)
	}

	// 使用 SHA256 哈希
	hashBytes := sha256.Sum256(objBytes)

	// 使用两个独立的哈希值进行双哈希
	hash1 := binary.BigEndian.Uint64(hashBytes[0:8])
	hash2 := binary.BigEndian.Uint64(hashBytes[8:16])

	indexes := make([]int64, bf.hashIterations)
	m := bf.size

	for i := 0; i < bf.hashIterations; i++ {
		combinedHash := hash1 + uint64(i)*hash2
		index := int64(combinedHash % uint64(m))
		indexes[i] = index
	}

	return indexes, nil
}

// optimalBloomParameters 计算布隆过滤器的大小和哈希迭代次数
func optimalBloomParameters(n int64, p float64) (size int64, hashIterations int) {
	if p <= 0.0 {
		p = 0.0001 // 最小假阳性率
	}
	m := -float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)
	k := int(math.Ceil((m / float64(n)) * math.Ln2))

	// 向上取整到最近的64的倍数以优化位操作
	m64 := int64(math.Ceil(m/64.0)) * 64

	return m64, k
}

// SetBit 设置位，如果位被设置返回 false，否则返回 true
func (bf *RedissonBloomFilter[T]) SetBit(offset int64, value bool) (bool, error) {
	var bitValue int
	if value {
		bitValue = 1
	} else {
		bitValue = 0
	}

	// 使用 BITSET 设置位
	result, err := bf.client.SetBit(context.Background(), bf.key, offset, bitValue).Result()
	if err != nil {
		return false, err
	}

	// 返回之前的位状态
	return result == 0, nil
}

// GetBit 获取位的值
func (bf *RedissonBloomFilter[T]) GetBit(offset int64) (bool, error) {
	result, err := bf.client.GetBit(context.Background(), bf.key, offset).Result()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// suffixName 构造配置键名
func suffixName(key, suffix string) string {
	return fmt.Sprintf("%s:%s", key, suffix)
}
