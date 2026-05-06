package redis

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type InMemoryStorage struct {
	mu     sync.RWMutex
	tokens map[string]time.Time
	log    *slog.Logger
}

func NewInMemory(log *slog.Logger) *InMemoryStorage {
	return &InMemoryStorage{
		tokens: make(map[string]time.Time),
		log:    log,
	}
}

func (s *InMemoryStorage) Add(ctx context.Context, token string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = time.Now().Add(ttl)
	s.log.Info("in-memory blacklist: token added", slog.String("token", token))
	return nil
}

func (s *InMemoryStorage) IsBlackListed(ctx context.Context, token string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tokens[token]

	return ok, nil
}
