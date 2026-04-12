package cache

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"time"

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
) (*models.RefreshToken, error) {
	storedToken, cacheErr := c.cache.RefreshToken(ctx, token)
	if cacheErr != nil {
		storedToken, nextErr := c.next.RefreshToken(ctx, token)
		if nextErr != nil {
			return nil, nextErr
		}

		return storedToken, nil
	}

	return storedToken, nil
}

func (c *tokenCache) SaveRefreshToken(
	ctx context.Context,
	userID int64,
	appID int,
	token string,
	expiresAt time.Time,
	ip netip.Addr,
) error {
	const op = "cache.TokenCache.SaveRefreshToken"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.SaveRefreshToken(ctx, userID, appID, token, expiresAt, ip)

	cacheErr := c.cache.SaveRefreshToken(ctx, userID, appID, token, expiresAt, ip)
	if cacheErr != nil {
		log.Error("failed to save token to cache", slog.Any("error", cacheErr))
	}

	return nextErr
}

func (c *tokenCache) RevokeToken(ctx context.Context, token string) error {
	const op = "cache.TokenCache.RevokeToken"

	log := c.log.With(slog.String("op", op))

	nextErr := c.next.RevokeToken(ctx, token)

	cacheErr := c.cache.RevokeToken(ctx, token)
	if cacheErr != nil {
		if !errors.Is(cacheErr, storage.ErrRefreshTokenNotFound) {
			log.Error("failed to delete token from cache", slog.Any("error", cacheErr))
		}
	}

	return nextErr
}

func (c *tokenCache) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	return c.next.RevokeAllUserTokens(ctx, userID)
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
