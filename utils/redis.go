package utils

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var Ctx = context.Background()

func InitRedis() error {
	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})

	_, err := RedisClient.Ping(Ctx).Result()
	if err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	fmt.Println("✅ Redis connected")
	return nil
}

func SetToken(key string, value string, ttl time.Duration) error {
	return RedisClient.Set(Ctx, key, value, ttl).Err()
}

func GetToken(key string) (string, error) {
	return RedisClient.Get(Ctx, key).Result()
}

func DeleteToken(key string) error {
	return RedisClient.Del(Ctx, key).Err()
}
