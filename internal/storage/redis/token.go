package redis

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
	"github.com/redis/rueidis"
)

const (
	tokenPrefix = "refresh-token:"
)

func getTokenKey(token string) string {
	return tokenPrefix + token
}

func (s *Storage) RefreshToken(
	ctx context.Context,
	token string,
) (*models.RefreshToken, error) {
	const op = "storage.redis.RefreshToken"

	key := getTokenKey(token)

	resp := s.client.Do(ctx, s.client.B().Get().Key(key).Build())

	// Handle "Key Not Found" specifically
	if rueidis.IsRedisNil(resp.Error()) {
		return nil, fmt.Errorf("%s: refresh token not found: %w", op, storage.ErrRefreshTokenNotFound)
	}

	if err := resp.Error(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var ip netip.Addr
	err = ip.UnmarshalBinary(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &models.RefreshToken{
		Token:      token,
		IP_address: ip,
	}, nil
}

func (s *Storage) SaveRefreshToken(
	ctx context.Context,
	userID int64,
	appId int,
	token string,
	expiresAt time.Time,
	ip netip.Addr,
) error {
	const op = "storage.redis.SaveRefreshToken"

	key := getTokenKey(token)
	data, err := ip.MarshalBinary()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	err = s.client.Do(ctx, s.client.B().Set().
		Key(key).
		Value(string(data)).
		Ex(s.cfg.RefreshTokenTTL).
		Build()).Error()

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) RevokeToken(ctx context.Context, token string) error {
	const op = "storage.redis.RevokeToken"

	key := getTokenKey(token)

	resp := s.client.Do(ctx, s.client.B().Del().Key(key).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("%s: internal failure: %w", op, err)
	}

	// Detect "No Such Key"
	count, _ := resp.AsInt64()
	if count == 0 {
		return fmt.Errorf("%s: refresh token not found: %w", op, storage.ErrRefreshTokenNotFound)
	}

	return nil
}

func (s *Storage) RevokeAllUserTokens(ctx context.Context, userId int64) error {
	return nil
}

func (s *Storage) RevokeAllAppTokens(ctx context.Context, appId int) error {
	return nil
}
