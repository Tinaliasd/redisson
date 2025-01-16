package redisson

import (
	"fmt"
	"testing"
	"time"
)

type User struct {
	ID   int
	Name string
}

func TestBloomFiltermain(t *testing.T) {

	// 创建布隆过滤器实例
	red := GetRedisson()
	bf := GetBloomFilter[User](red, "user_bloom_filter")

	// 初始化布隆过滤器
	initialized := bf.TryInit(1000000, 0.01)
	if initialized {
		fmt.Println("Bloom filter initialized successfully.")
	} else {
		fmt.Println("Bloom filter was already initialized.")
	}

	// 添加元素
	user := User{ID: 1, Name: "Alice"}
	added := bf.Add(user)
	if added {
		fmt.Println("User added to Bloom filter.")
	} else {
		fmt.Println("User already exists in Bloom filter.")
	}

	// 检查元素
	exists := bf.Contains(user)
	if exists {
		fmt.Println("User exists in Bloom filter.")
	} else {
		fmt.Println("User does not exist in Bloom filter.")
	}

	// 获取布隆过滤器信息
	fmt.Printf("Expected Insertions: %d\n", bf.GetExpectedInsertions())
	fmt.Printf("False Probability: %f\n", bf.GetFalseProbability())
	fmt.Printf("Size: %d bits\n", bf.GetSize())
	fmt.Printf("Hash Iterations: %d\n", bf.GetHashIterations())

	// 估算已添加的元素数量
	count := bf.Count()
	fmt.Printf("Estimated number of inserted elements: %d\n", count)

	// 设置过期时间
	_, err := bf.Expire(24 * time.Hour)
	if err != nil {
		fmt.Printf("Error setting expiration: %v\n", err)
	} else {
		fmt.Println("Bloom filter will expire in 24 hours.")
	}
}
