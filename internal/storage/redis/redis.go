package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
	"github.com/redis/rueidis"
)

const (
	appPrefix = "app:"
)

type Config struct {
	Host string
	Port int

	AppTTL time.Duration
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

func getAppKey(appId int) string {
	return appPrefix + strconv.Itoa(appId)
}

// App provides app
func (s *Storage) App(ctx context.Context, appID int) (*models.App, error) {
	const op = "storage.redis.App"

	key := getAppKey(appID)

	resp := s.client.Do(ctx, s.client.B().Get().Key(key).Build())

	// Handle "Key Not Found" specifically
	if rueidis.IsRedisNil(resp.Error()) {
		return nil, fmt.Errorf("%s: app not found: %w", op, storage.ErrAppNotFound)
	}

	if err := resp.Error(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &models.App{
		ID:     appID,
		Secret: data,
	}, nil
}

// SaveApp saves app
func (s *Storage) SaveApp(ctx context.Context, app *models.App) (id int, err error) {
	const op = "storage.redis.SaveApp"

	key := getAppKey(app.ID)

	err = s.client.Do(ctx, s.client.B().Set().
		Key(key).
		Value(string(app.Secret)).
		Ex(s.cfg.AppTTL).
		Build()).Error()

	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return app.ID, nil
}

// DeleteApp deletes app
func (s *Storage) DeleteApp(ctx context.Context, appID int) error {
	const op = "storage.redis.DeleteApp"

	key := getAppKey(appID)

	resp := s.client.Do(ctx, s.client.B().Del().Key(key).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("%s: internal failure: %w", op, err)
	}

	// Detect "No Such Key"
	count, _ := resp.AsInt64()
	if count == 0 {
		return fmt.Errorf("%s: app not found: %w", op, storage.ErrAppNotFound)
	}

	return nil
}
