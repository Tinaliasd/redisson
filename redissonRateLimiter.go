package redisson

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

// =============== RateLimiter 相关定义 ===============

type RateType int

const (
	RateTypeOVERALL    RateType = iota // 0 => 所有实例共享
	RateTypePER_CLIENT                 // 1 => 仅本客户端限流
)

// MarshalBinary implements encoding.BinaryMarshaler
func (rl RateType) MarshalBinary() ([]byte, error) {
	return []byte(strconv.Itoa(int(rl))), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (rl *RateType) UnmarshalBinary(data []byte) error {
	val, err := strconv.Atoi(string(data))
	if err != nil {
		return err
	}
	*rl = RateType(val)
	return nil
}

// 时间单位
type RateIntervalUnit int

const (
	Milliseconds RateIntervalUnit = iota
	Seconds
	Minutes
	Hours
	Days
)

func (unit RateIntervalUnit) ToMillis(value int64) int64 {
	switch unit {
	case Milliseconds:
		return value
	case Seconds:
		return value * 1000
	case Minutes:
		return value * 60 * 1000
	case Hours:
		return value * 60 * 60 * 1000
	case Days:
		return value * 24 * 60 * 60 * 1000
	default:
		return 0
	}
}

// RateLimiterConfig 封装限流配置
type RateLimiterConfig struct {
	RateType     RateType
	RateInterval int64 // 毫秒
	Rate         int64 // 速率(令牌桶容量)
}

// RRateLimiter 接口
type RRateLimiter interface {
	RExpirable

	// TrySetRate 初始化限流器的配置，并将配置存储到 Redis 服务器。
	// 如果设置成功，返回 true，否则返回 false。
	TrySetRate(mode RateType, rate, rateInterval int64, unit RateIntervalUnit) (bool, error)

	// SetRate 更新限流器的配置，并将配置存储到 Redis 服务器。
	SetRate(mode RateType, rate, rateInterval int64, unit RateIntervalUnit) error

	// TryAcquire 尝试获取一个许可，如果成功则返回 true，否则返回 false。
	TryAcquire() (bool, error)

	// TryAcquirePermits 尝试获取指定数量的许可，如果成功则返回 true，否则返回 false。
	TryAcquirePermits(permits int64) (bool, error)

	// Acquire 获取一个许可，阻塞直到成功。
	Acquire() error

	// AcquirePermits 获取指定数量的许可，阻塞直到成功。
	AcquirePermits(permits int64) error

	// TryAcquireWithTimeout 尝试在指定时间内获取一个许可，如果成功则返回 true，否则返回 false。
	TryAcquireWithTimeout(timeout time.Duration) (bool, error)

	// TryAcquirePermitsWithTimeout 尝试在指定时间内获取指定数量的许可，如果成功则返回 true，否则返回 false。
	TryAcquirePermitsWithTimeout(permits int64, timeout time.Duration) (bool, error)

	// GetConfig 返回当前限流器的配置。
	GetConfig() (*RateLimiterConfig, error)

	// AvailablePermits 返回当前可用的许可数量。
	AvailablePermits() (int64, error)
}

// =============== 具体的限流器实现 ===============

type RedissonRateLimiter struct {
	*RedissonExpirable
	name string
}

// getPermitsName 返回全局许可键名。
func (rl *RedissonRateLimiter) getPermitsName() string {
	return rl.suffixName(rl.getRawName(), "permits")
}

// getClientPermitsName 返回客户端许可键名。
func (rl *RedissonRateLimiter) getClientPermitsName() string {
	// 假设在此直接使用 rl.RedissonObject.Redisson.id，
	// 对应 Java 中的 commandExecutor.getConnectionManager().getId()。
	return rl.suffixName(rl.getPermitsName(), rl.Redisson.id)
}

// getValueName 返回全局值键名。
func (rl *RedissonRateLimiter) getValueName() string {
	return rl.suffixName(rl.getRawName(), "value")
}

// getClientValueName 返回客户端值键名。
func (rl *RedissonRateLimiter) getClientValueName() string {
	return rl.suffixName(rl.getValueName(), rl.Redisson.id)
}

// 构造函数
func newRedissonRateLimiter(name string, redisson *Redisson) RRateLimiter {
	return &RedissonRateLimiter{
		RedissonExpirable: newRedissonExpirable(name, redisson),
		name:              name,
	}
}

// 一些在 Lua 中会用到的 key 约定
// 注意：在 Redisson 中，会区分 Overall 与 PerClient 两种 key（带 clientID 后缀）
func (rl *RedissonRateLimiter) configHashKey() string {
	return rl.getRawName() // 存放 config (rate, interval, type) 的 hash
}

// 令牌余量
func (rl *RedissonRateLimiter) valueKey() string {
	// suffixName("{foo}", "value") => {foo}:value
	return rl.suffixName(rl.getRawName(), "value")
}

func (rl *RedissonRateLimiter) clientValueKey() string {
	// rl.g. {foo}:value:client123
	return rl.suffixName(rl.valueKey(), rl.id)
}

func (rl *RedissonRateLimiter) permitsKey() string {
	return rl.suffixName(rl.getRawName(), "permits")
}

func (rl *RedissonRateLimiter) clientPermitsKey() string {
	// rl.g. {foo}:permits:client123
	return rl.suffixName(rl.permitsKey(), rl.id)
}

// =============== 接口方法实现 ===============

// TrySetRate
func (rl *RedissonRateLimiter) TrySetRate(mode RateType, rate, rateInterval int64, unit RateIntervalUnit) (bool, error) {

	res, err := rl.trySetRateLua(mode, rate, rateInterval, unit)
	if err != nil {
		return false, err
	}
	// trySetRateScript 最后返回的是 0 or 1
	return *res == 1, nil

}

func (rl *RedissonRateLimiter) trySetRateLua(mode RateType, rate, rateInterval int64, unit RateIntervalUnit) (*int64, error) {
	ctx := context.Background()
	keys := []string{rl.configHashKey()}
	args := []interface{}{
		rate,
		unit.ToMillis(rateInterval),
		mode, // 0 或 1
	}
	res, err := rl.client.Eval(ctx, trySetRateScript, keys, args...).Int64()

	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &res, err
}

// SetRate
func (rl *RedissonRateLimiter) SetRate(mode RateType, rate, rateInterval int64, unit RateIntervalUnit) error {
	_, err := rl.setRateLua(mode, rate, rateInterval, unit)

	return err
}

func (rl *RedissonRateLimiter) setRateLua(mode RateType, rate, rateInterval int64, unit RateIntervalUnit) (*int64, error) {
	ctx := context.Background()
	keys := []string{
		rl.configHashKey(),
		rl.valueKey(),
		rl.permitsKey(),
	}
	args := []interface{}{
		rate,
		unit.ToMillis(rateInterval),
		mode,
	}
	res, err := rl.client.Eval(ctx, setRateScript, keys, args...).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &res, err
}

// TryAcquire
// 简化：等价于获取 1 个许可
func (rl *RedissonRateLimiter) TryAcquire() (bool, error) {
	return rl.TryAcquirePermits(1)
}

//	func (rl *RedissonRateLimiter) TryAcquirePermits(permits int64) (bool, error) {
//		waitTime, err := rl.tryAcquireLua(permits)
//		if err != nil {
//			return false, err
//		}
//
//		if waitTime == nil {
//			return true, nil
//		} else {
//			return false, nil
//		}
//	}
func (rl *RedissonRateLimiter) TryAcquirePermits(permits int64) (bool, error) {
	fmt.Printf("Attempting to acquire %d permits...\n", permits)
	waitTime, err := rl.tryAcquireLua(permits)
	if err != nil {
		fmt.Printf("Error in TryAcquirePermits: %v\n", err)
		return false, err
	}

	if waitTime == nil {
		fmt.Println("Permits acquired successfully.")
		return true, nil
	} else {
		fmt.Printf("Not enough permits available, need to wait %d ms.\n", *waitTime)
		return false, nil
	}
}

// Acquire
// 简化实现：循环调用 TryAcquire，如果不成功就阻塞等待
func (rl *RedissonRateLimiter) Acquire() error {
	return rl.AcquirePermits(1)
}

// AcquirePermits
func (rl *RedissonRateLimiter) AcquirePermits(permits int64) error {
	_, err := rl.TryAcquirePermitsWithTimeout(permits, -1)
	return err
}

// TryAcquireWithTimeout
func (rl *RedissonRateLimiter) TryAcquireWithTimeout(timeout time.Duration) (bool, error) {
	return rl.TryAcquirePermitsWithTimeout(1, timeout)
}

// TryAcquirePermitsWithTimeout 参考 Java 中的逻辑：
// 1. 先尝试获取令牌；
// 2. 若立即可获取 (delay == nil), 返回 true；
// 3. 若返回 delay，需要判断 timeout；
//   - 若 timeout < 0，表示无限等待，则等待 delay 毫秒后再次尝试；
//   - 若有超时时间，则看是否还有剩余等待时间；
//   - 若剩余等待时间 < 0，直接返回 false；
//   - 若剩余等待时间 < delay，等待到期后返回 false；
//   - 否则等待 delay 后再次递归尝试，直到超时或成功。
func (rl *RedissonRateLimiter) TryAcquirePermitsWithTimeout(permits int64, timeout time.Duration) (bool, error) {
	start := time.Now()
	timeWait, err := rl.tryAcquireLua(permits)
	if err != nil {
		return false, err
	}

	if timeWait == nil { // 先检查是否为 nil
		return true, nil // 可以立即获取许可
	}

	delayMs := *timeWait // 确保 timeWait 不为 nil 后再解引用

	// 脚本返回了 delay，需要根据 timeout 判断是否再次调度
	if timeout < 0 {
		// 等待 delay 后再无限重试
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		return rl.TryAcquirePermitsWithTimeout(permits, timeout)
	}

	// 有超时时间，计算剩余时间
	elapsed := time.Since(start)
	remains := timeout - elapsed
	if remains <= 0 {
		// 超时
		return false, nil
	}

	// 如果剩余时间小于本次返回的 delay，则等待到期后返回 false
	delayDuration := time.Duration(delayMs) * time.Millisecond
	if remains < delayDuration {
		time.Sleep(remains)
		return false, nil
	}

	// 否则可等待 delay，再次尝试
	time.Sleep(delayDuration)

	// 等待完 delay 后可能又经过了一小段时间，需再次计算剩余
	newElapsed := time.Since(start)
	newRemains := timeout - newElapsed
	if newRemains <= 0 {
		return false, nil
	}

	return rl.TryAcquirePermitsWithTimeout(permits, newRemains)
}

// GetConfig
func (rl *RedissonRateLimiter) GetConfig() (*RateLimiterConfig, error) {
	ctx := context.Background()
	h, err := rl.client.HGetAll(ctx, rl.configHashKey()).Result()
	if err != nil {
		return nil, err
	}
	if len(h) == 0 {
		return nil, errors.New("RateLimiter is not initialized")
	}
	rate, _ := strconv.ParseInt(h["rate"], 10, 64)
	interval, _ := strconv.ParseInt(h["interval"], 10, 64)
	typ, _ := strconv.ParseInt(h["type"], 10, 64)
	return &RateLimiterConfig{
		RateType:     RateType(typ),
		RateInterval: interval,
		Rate:         rate,
	}, nil
}

// AvailablePermits
func (rl *RedissonRateLimiter) AvailablePermits() (int64, error) {
	fmt.Println("Fetching available permits...")
	res, err := rl.availablePermitsLua()
	if err != nil {
		//fmt.Printf("Error fetching available permits: %v\n", err)
		//return 0, err
		return 0, fmt.Errorf("failed to get available permits: %v", err)
	}
	if res == nil {
		return 0, errors.New("rate limiter not initialized")
	}
	fmt.Printf("Available permits: %d\n", *res)
	return *res, nil
}

func (rl *RedissonRateLimiter) availablePermitsLua() (*int64, error) {
	ctx := context.Background()
	keys := []string{
		rl.configHashKey(),
		rl.valueKey(),
		rl.clientValueKey(),
		rl.permitsKey(),
		rl.clientPermitsKey(),
	}
	args := []interface{}{
		time.Now().UnixMilli(),
	}
	res, err := rl.client.Eval(ctx, availablePermitsScript, keys, args...).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return &res, err

}

// func (rl *RedissonRateLimiter) tryAcquireLua(permits int64) (*int64, error) {
//
//		// keys 数组与 Lua 中的 KEYS 对应，顺序需与脚本中一致
//		keys := []string{
//			rl.getRawName(),
//			rl.getValueName(),
//			rl.getClientValueName(),
//			rl.getPermitsName(),
//			rl.getClientPermitsName(),
//		}
//
//		// ARGV:
//		// 1) 请求的令牌数 permits
//		// 2) 当前时间毫秒时间戳
//		// 3) 随机字节 (Java 原代码是 8 字节随机值)
//		nowMillis := time.Now().UnixNano() / int64(time.Millisecond)
//		randomBytes := make([]byte, 8)
//		_, err := rand.Read(randomBytes)
//		if err != nil {
//			return nil, err
//		}
//
//		args := []interface{}{
//			permits,
//			nowMillis,
//			string(randomBytes), // Lua 脚本里会把这作为二进制字符串处理
//		}
//
//		// 执行 Lua 脚本
//		res, err := rl.client.Eval(context.Background(), tryAcquireScript, keys, args...).Int64()
//		if err != nil {
//			if err == redis.Nil {
//				return nil, nil
//			}
//			return nil, err
//		}
//
//		return &res, nil
//	}
//func (rl *RedissonRateLimiter) tryAcquireLua(permits int64) (*int64, error) {
//	fmt.Printf("Executing tryAcquireLua for %d permits...\n", permits)
//
//	keys := []string{
//		rl.getRawName(),
//		rl.getValueName(),
//		rl.getClientValueName(),
//		rl.getPermitsName(),
//		rl.getClientPermitsName(),
//	}
//
//	nowMillis := time.Now().UnixNano() / int64(time.Millisecond)
//	randomBytes := make([]byte, 8)
//	_, err := rand.Read(randomBytes)
//	if err != nil {
//		fmt.Printf("Error generating random bytes: %v\n", err)
//		return nil, err
//	}
//
//	args := []interface{}{
//		permits,
//		nowMillis,
//		string(randomBytes),
//	}
//
//	// Printing keys
//	fmt.Println("Keys:")
//	for _, key := range keys {
//		fmt.Println(key)
//	}
//
//	// Printing args
//	fmt.Println("Args:")
//	for _, arg := range args {
//		switch v := arg.(type) {
//		case int:
//			fmt.Println("Integer:", v)
//		case int64:
//			fmt.Println("Int64:", v)
//		case []byte:
//			fmt.Println("Byte slice in hex:", hex.EncodeToString(v))
//		case string:
//			fmt.Printf("randomBytes in hex: %x\n", randomBytes)
//		default:
//			fmt.Printf("Unknown type: %T with value: %v\n", v, v)
//		}
//	}
//
//	res, err := rl.client.Eval(context.Background(), tryAcquireScript, keys, args...).Int64()
//	if err != nil {
//		if err == redis.Nil {
//			fmt.Println("Redis returned nil.")
//			return nil, nil
//		}
//		fmt.Printf("Error executing Lua script: %v\n", err)
//		return nil, err
//	}
//
//	fmt.Printf("Lua script result: %d\n", res)
//	return &res, nil
//}

func (rl *RedissonRateLimiter) tryAcquireLua(permits int64) (*int64, error) {
	// 加锁保护并发访问

	keys := []string{
		rl.getRawName(),
		rl.getValueName(),
		rl.getClientValueName(),
		rl.getPermitsName(),
		rl.getClientPermitsName(),
	}

	//nowMillis := time.Now().UnixNano() / int64(time.Millisecond)

	nowMillis := time.Now().UnixMilli()
	// 使用更安全的随机数生成
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %v", err)
	}

	args := []interface{}{
		permits,
		nowMillis,
		hex.EncodeToString(randomBytes), // 使用 hex 编码确保安全传输
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := rl.client.Eval(ctx, tryAcquireScript, keys, args...).Int64()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to execute rate limit script: %v", err)
	}

	return &res, nil
}

// =============== Lua 脚本（示例） ===============

const tryAcquireScript = `
local rate = redis.call('hget', KEYS[1], 'rate');
local interval = redis.call('hget', KEYS[1], 'interval');
local type = redis.call('hget', KEYS[1], 'type');
assert(rate ~= false and interval ~= false and type ~= false, 'RateLimiter is not initialized')

local valueName = KEYS[2];
local permitsName = KEYS[4];
if type == '1' then 
valueName = KEYS[3];
permitsName = KEYS[5];
end;

assert(tonumber(rate) >= tonumber(ARGV[1]), 'Requested permits amount could not exceed defined rate'); 

local currentValue = redis.call('get', valueName); 
local res;
if currentValue ~= false then 
local expiredValues = redis.call('zrangebyscore', permitsName, 0, tonumber(ARGV[2]) - interval); 
local released = 0; 
for i, v in ipairs(expiredValues) do 
local random, permits = struct.unpack('Bc0I', v);
released = released + permits;
end; 

if released > 0 then 
redis.call('zremrangebyscore', permitsName, 0, tonumber(ARGV[2]) - interval); 
if tonumber(currentValue) + released > tonumber(rate) then 
currentValue = tonumber(rate) - redis.call('zcard', permitsName); 
else 
currentValue = tonumber(currentValue) + released; 
end; 
redis.call('set', valueName, currentValue);
end;

if tonumber(currentValue) < tonumber(ARGV[1]) then 
local firstValue = redis.call('zrange', permitsName, 0, 0, 'withscores'); 
res = 3 + interval - (tonumber(ARGV[2]) - tonumber(firstValue[2]));
else 
redis.call('zadd', permitsName, ARGV[2], struct.pack('Bc0I', string.len(ARGV[3]), ARGV[3], ARGV[1])); 
redis.call('decrby', valueName, ARGV[1]); 
res = nil; 
end; 
else 
redis.call('set', valueName, rate); 
redis.call('zadd', permitsName, ARGV[2], struct.pack('Bc0I', string.len(ARGV[3]), ARGV[3], ARGV[1])); 
redis.call('decrby', valueName, ARGV[1]); 
res = nil; 
end;

local ttl = redis.call('pttl', KEYS[1]); 
if ttl > 0 then 
redis.call('pexpire', valueName, ttl); 
redis.call('pexpire', permitsName, ttl); 
end; 
return res;
`

// setRateScript：覆盖写入配置
const setRateScript = `
redis.call('hset', KEYS[1], 'rate', ARGV[1]);
redis.call('hset', KEYS[1], 'interval', ARGV[2]);
redis.call('hset', KEYS[1], 'type', ARGV[3]);
redis.call('del', KEYS[2], KEYS[3]);
`

// trySetRateScript：只有当还没设置过的时候才写入
const trySetRateScript = `
redis.call('hsetnx', KEYS[1], 'rate', ARGV[1]);
redis.call('hsetnx', KEYS[1], 'interval', ARGV[2]);
return redis.call('hsetnx', KEYS[1], 'type', ARGV[3]);
`

// availablePermitsScript：移除过期令牌后，返回当前余量
// availablePermitsScript：移除过期令牌后，返回当前余量
const availablePermitsScript = `
local rate = redis.call('hget', KEYS[1], 'rate');
local interval = redis.call('hget', KEYS[1], 'interval');
local type = redis.call('hget', KEYS[1], 'type');
assert(rate ~= false and interval ~= false and type ~= false, 'RateLimiter is not initialized');

local valueName = KEYS[2];
local permitsName = KEYS[4];
if type == '1' then
   valueName = KEYS[3];
   permitsName = KEYS[5];
end;

local currentValue = redis.call('get', valueName);
if currentValue == false then
   redis.call('set', valueName, rate);
   return rate;
else
   -- 移除过期
   local expiredValues = redis.call('zrangebyscore', permitsName, 0, tonumber(ARGV[1]) - interval);
   local released = 0;
   for i, v in ipairs(expiredValues) do
       local random, permits = struct.unpack('Bc0I', v);
       released = released + permits;
   end;

   if released > 0 then
       redis.call('zremrangebyscore', permitsName, 0, tonumber(ARGV[1]) - interval);
       currentValue = tonumber(currentValue) + released;
       redis.call('set', valueName, currentValue);
   end;

   return currentValue;
end;
`
