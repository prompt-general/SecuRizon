package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/go-redis/redis/v9"
)

type RedisCache struct {
    client *redis.Client
    prefix string
    ttl    time.Duration
}

func NewRedisCache(addr string, password string, db int, prefix string) *RedisCache {
    client := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,

        // Connection pool settings
        PoolSize:     100,
        MinIdleConns: 10,
        MaxConnAge:   30 * time.Minute,

        // Timeouts
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolTimeout:  4 * time.Second,

        // Circuit breaker
        MaxRetries:      3,
        MinRetryBackoff: 8 * time.Millisecond,
        MaxRetryBackoff: 512 * time.Millisecond,
    })

    return &RedisCache{
        client: client,
        prefix: prefix,
        ttl:    5 * time.Minute,
    }
}

func (rc *RedisCache) Get(ctx context.Context, key string, target interface{}) (bool, error) {
    fullKey := rc.prefix + ":" + key

    data, err := rc.client.Get(ctx, fullKey).Bytes()
    if err == redis.Nil {
        return false, nil
    }
    if err != nil {
        return false, fmt.Errorf("redis get failed: %v", err)
    }

    if err := json.Unmarshal(data, target); err != nil {
        return false, fmt.Errorf("failed to unmarshal cached data: %v", err)
    }

    return true, nil
}

func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    fullKey := rc.prefix + ":" + key

    data, err := json.Marshal(value)
    if err != nil {
        return fmt.Errorf("failed to marshal value: %v", err)
    }

    if ttl == 0 {
        ttl = rc.ttl
    }

    err = rc.client.Set(ctx, fullKey, data, ttl).Err()
    if err != nil {
        return fmt.Errorf("redis set failed: %v", err)
    }

    return nil
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
    fullKey := rc.prefix + ":" + key
    return rc.client.Del(ctx, fullKey).Err()
}

func (rc *RedisCache) GetOrSet(ctx context.Context, key string, ttl time.Duration,
    fetchFunc func() (interface{}, error)) (interface{}, error) {

    // Try to get from cache
    var cached interface{}
    found, err := rc.Get(ctx, key, &cached)
    if err != nil {
        return nil, err
    }
    if found {
        return cached, nil
    }

    // Not in cache, fetch from source
    value, err := fetchFunc()
    if err != nil {
        return nil, err
    }

    // Store in cache
    if err := rc.Set(ctx, key, value, ttl); err != nil {
        // Log but don't fail if cache set fails
        fmt.Printf("Failed to set cache: %v\n", err)
    }

    return value, nil
}

// Cache patterns for SecuRizon
type CacheManager struct {
    redisCache *RedisCache
    localCache *sync.Map
    stats      CacheStats
}

type CacheStats struct {
    hits   int64
    misses int64
    errors int64
}

// Placeholder type definition
type AttackPath struct {
    ID string `json:"id"`
    // other fields specific to your AttackPath model
}

func NewCacheManager(redisAddr string) *CacheManager {
    return &CacheManager{
        redisCache: NewRedisCache(redisAddr, "", 0, "securazion"),
        localCache: &sync.Map{},
    }
}

func (cm *CacheManager) GetAttackPaths(ctx context.Context, assetID string, maxHops int) ([]AttackPath, error) {
    cacheKey := fmt.Sprintf("attack-paths:%s:%d", assetID, maxHops)

    // Try local cache first
    if cached, ok := cm.localCache.Load(cacheKey); ok {
        cm.stats.hits++
        return cached.([]AttackPath), nil
    }

    // Try Redis cache
    var paths []AttackPath
    found, err := cm.redisCache.Get(ctx, cacheKey, &paths)
    if err != nil {
        cm.stats.errors++
    } else if found {
        cm.stats.hits++
        // Store in local cache
        cm.localCache.Store(cacheKey, paths)
        return paths, nil
    }

    // Cache miss - compute from database
    cm.stats.misses++
    paths, err = cm.computeAttackPaths(ctx, assetID, maxHops)
    if err != nil {
        return nil, err
    }

    // Store in both caches
    cm.localCache.Store(cacheKey, paths)
    cm.redisCache.Set(ctx, cacheKey, paths, 2*time.Minute)

    return paths, nil
}

func (cm *CacheManager) computeAttackPaths(ctx context.Context, assetID string, maxHops int) ([]AttackPath, error) {
    // Placeholder logic for computing attack paths
    // In a real implementation this would query Neo4j or another source
    return []AttackPath{}, nil
}

func (cm *CacheManager) InvalidateAsset(ctx context.Context, assetID string) {
    // Invalidate all cache entries for this asset
    pattern := fmt.Sprintf("%s:attack-paths:%s:*", cm.redisCache.prefix, assetID)

    keys, err := cm.redisCache.client.Keys(ctx, pattern).Result()
    if err == nil {
        for _, key := range keys {
            // Remove prefix when calling Delete since it adds it again
            // For Keys() we need the full pattern unfortunately
            // Here we use the client directly to delete the exact key
            cm.redisCache.client.Del(ctx, key)
            
            // Also remove from local cache - here we ideally need the base key
            // This simplification might need adjustment for exact key matching
            cm.localCache.Delete(key)
        }
    }

    // Invalidate related cache entries
    relatedKeys := []string{
        fmt.Sprintf("asset:%s", assetID),
        fmt.Sprintf("asset-relationships:%s", assetID),
        fmt.Sprintf("risk-score:%s", assetID),
    }

    for _, key := range relatedKeys {
        cm.redisCache.Delete(ctx, key)
        cm.localCache.Delete(key)
    }
}
