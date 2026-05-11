package redis

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
	"github.com/redis/rueidis"
)

const (
	appPrefix = "app:"
)

func getAppKey(appID uuid.UUID) string {
	return appPrefix + appID.String()
}

// App provides app
func (s *Storage) App(ctx context.Context, appID uuid.UUID) (*models.App, error) {
	const op = "storage.redis.App"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getAppKey(appID)

	resp := s.client.Do(ctxOp, s.client.B().Get().Key(key).Build())

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

	if err := s.client.Do(
		ctxOp,
		s.client.B().
			Expire().
			Key(key).
			Seconds(int64(s.cfg.AppTTL.Seconds())).
			Build(),
	).Error(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &models.App{
		ID:     appID,
		Secret: data,
	}, nil
}

// SaveApp saves app
func (s *Storage) SaveApp(ctx context.Context, app *models.App) (id uuid.UUID, err error) {
	const op = "storage.redis.SaveApp"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getAppKey(app.ID)

	err = s.client.Do(ctxOp, s.client.B().Set().
		Key(key).
		Value(string(app.Secret)).
		Ex(s.cfg.AppTTL).
		Build()).Error()

	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	return app.ID, nil
}

// DeleteApp deletes app
func (s *Storage) DeleteApp(ctx context.Context, appID uuid.UUID) error {
	const op = "storage.redis.DeleteApp"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getAppKey(appID)

	resp := s.client.Do(ctxOp, s.client.B().Del().Key(key).Build())
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
