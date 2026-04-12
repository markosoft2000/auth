package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/rueidis"
)

type Config struct {
	Host string
	Port int

	AppTTL          time.Duration
	RefreshTokenTTL time.Duration
}

type Storage struct {
	client rueidis.Client
	cfg    Config
}

func New(cfg Config) (*Storage, error) {
	const op = "storage.redis.New"

	c, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{
			fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := c.Do(context.Background(), c.B().Ping().Build()).Error(); err != nil {
		return nil, fmt.Errorf("%s: could not ping redis: %w", op, err)
	}

	return &Storage{
		client: c,
		cfg:    cfg,
	}, nil
}

func (s *Storage) Stop() {
	s.client.Close()
}
