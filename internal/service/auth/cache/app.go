package cache

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/markosoft2000/auth/internal/domain/models"
	auth "github.com/markosoft2000/auth/internal/service/auth"
	"github.com/markosoft2000/auth/internal/storage"
)

type appCache struct {
	log *slog.Logger

	cache auth.AppManager
	next  auth.AppManager
}

func NewAppCache(log *slog.Logger, cache auth.AppManager, next auth.AppManager) auth.AppManager {
	return &appCache{
		log:   log,
		cache: cache,
		next:  next,
	}
}

func (c *appCache) App(ctx context.Context, appID uuid.UUID) (*models.App, error) {
	const op = "cache.AppCache.App"

	log := c.log.With(slog.String("op", op))

	app, err := c.cache.App(ctx, appID)
	if err != nil {
		app, nextErr := c.next.App(ctx, appID)
		if nextErr != nil {
			return nil, nextErr
		}

		_, cacheErr := c.cache.SaveApp(ctx, app)
		if cacheErr != nil {
			log.Error("failed to save app to cache", slog.Any("error", cacheErr))
		}

		return app, nil
	}

	return app, nil
}

func (c *appCache) SaveApp(ctx context.Context, app *models.App) (id uuid.UUID, err error) {
	return c.next.SaveApp(ctx, app)
}

func (c *appCache) DeleteApp(ctx context.Context, appID uuid.UUID) error {
	const op = "cache.AppCache.DeleteApp"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.DeleteApp(ctx, appID)

	cacheErr := c.cache.DeleteApp(ctx, appID)
	if cacheErr != nil {
		if !errors.Is(cacheErr, storage.ErrAppNotFound) {
			log.Error("failed to delete app from cache", slog.Any("error", cacheErr))
		}
	}

	return nextErr
}
