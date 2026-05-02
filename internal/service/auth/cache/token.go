package cache

import (
	"context"
	"errors"
	"log/slog"

	"github.com/markosoft2000/auth/internal/domain/models"
	_ "github.com/markosoft2000/auth/internal/domain/models"
	auth "github.com/markosoft2000/auth/internal/service/auth"
	"github.com/markosoft2000/auth/internal/storage"
)

type tokenCache struct {
	log *slog.Logger

	cache auth.TokenManager
	next  auth.TokenManager
}

func NewTokenCache(log *slog.Logger, cache auth.TokenManager, next auth.TokenManager) auth.TokenManager {
	return &tokenCache{
		log:   log,
		cache: cache,
		next:  next,
	}
}

func (c *tokenCache) RefreshToken(
	ctx context.Context,
	token string,
	userID int64,
	appID int,
) (*models.RefreshToken, error) {
	if token == "" {
		return c.next.RefreshToken(ctx, token, userID, appID)
	}

	storedToken, cacheErr := c.cache.RefreshToken(ctx, token, userID, appID)
	if cacheErr != nil || storedToken == nil || storedToken.Token == "" {
		storedToken, nextErr := c.next.RefreshToken(ctx, token, userID, appID)
		if nextErr != nil {
			return nil, nextErr
		}

		go c.cache.SaveRefreshToken(ctx, storedToken)

		return storedToken, nil
	}

	return storedToken, nil
}

func (c *tokenCache) SaveRefreshToken(
	ctx context.Context,
	token *models.RefreshToken,
) error {
	const op = "cache.TokenCache.SaveRefreshToken"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.SaveRefreshToken(ctx, token)

	go func() {
		cacheErr := c.cache.SaveRefreshToken(ctx, token)
		if cacheErr != nil {
			log.Error("failed to save token to cache", slog.Any("error", cacheErr))
		}
	}()

	return nextErr
}

func (c *tokenCache) RevokeToken(
	ctx context.Context,
	token string,
	userID int64,
	appID int,
) error {
	const op = "cache.TokenCache.RevokeToken"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.RevokeToken(ctx, token, userID, appID)

	cacheErr := c.cache.RevokeToken(ctx, token, userID, appID)
	if cacheErr != nil {
		if !errors.Is(cacheErr, storage.ErrRefreshTokenNotFound) {
			log.Error("failed to delete token from cache", slog.Any("error", cacheErr))
		}
	}

	return nextErr
}

func (c *tokenCache) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	const op = "cache.TokenCache.RevokeAllUserTokens"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.RevokeAllUserTokens(ctx, userID)

	cacheErr := c.cache.RevokeAllUserTokens(ctx, userID)
	if cacheErr != nil {
		log.Error("failed to revoke all tokens for user in cache", slog.Any("error", cacheErr))
	}

	return nextErr
}

func (c *tokenCache) RevokeAllAppTokens(ctx context.Context, appID int) error {
	const op = "cache.TokenCache.RevokeAllAppTokens"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.RevokeAllAppTokens(ctx, appID)

	cacheErr := c.cache.RevokeAllAppTokens(ctx, appID)
	if cacheErr != nil {
		log.Error("failed to revoke all tokens for app in cache", slog.Any("error", cacheErr))
	}

	return nextErr
}
