package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Storage struct {
	client *redis.Client
}

func New(address string) (*Storage, error) {
	const op = "storage.redis.New"

	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "",
		DB:       0,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Storage{client: client}, nil
}

func (s *Storage) Add(ctx context.Context, token string, ttl time.Duration) error {
	const op = "storage.redis.Add"

	err := s.client.Set(ctx, token, true, ttl).Err()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)

	}

	return nil
}

func (s *Storage) IsBlackListed(ctx context.Context, token string) (bool, error) {
	const op = "storage.redis.IsBlackListed"

	res, err := s.client.Exists(ctx, token).Result()
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return res > 0, nil
}
