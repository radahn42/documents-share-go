package cache

import (
	"context"
	"document-server/internal/config"
	"document-server/internal/domain/services"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(cfg config.RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	result := r.client.Get(ctx, key)
	if result.Err() != nil {
		return "", result.Err()
	}
	return result.Val(), nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value any, duration time.Duration) error {
	return r.client.Set(ctx, key, value, duration).Err()
}

func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	result := r.client.Keys(ctx, pattern)
	if result.Err() != nil {
		return nil, result.Err()
	}
	return result.Val(), nil
}

var _ services.RedisClient = (*RedisCache)(nil)
