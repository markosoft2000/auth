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
	tokenPrefix  = "refresh-token:"
	appTagPrefix = "tag:"

	deleteAppTokenLimit = 100
)

func getTokenKey(token string) string {
	return tokenPrefix + token
}

func getAppTagKey(appID int) string {
	return appTagPrefix + getAppKey(appID)
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
		return nil, fmt.Errorf(
			"%s: refresh token not found: %w",
			op,
			storage.ErrRefreshTokenNotFound,
		)
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
	appID int,
	token string,
	expiresAt time.Time,
	ip netip.Addr,
) error {
	const op = "storage.redis.SaveRefreshToken"

	key := getTokenKey(token)
	tag := getAppTagKey(appID)
	data, err := ip.MarshalBinary()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	// 1. Save the token
	// 2. Add the token key to the app's tag set
	cmds := make(rueidis.Commands, 0, 2)
	cmds = append(
		cmds,
		s.client.B().Set().
			Key(key).
			Value(string(data)).
			Ex(s.cfg.RefreshTokenTTL).
			Build(),
	)
	cmds = append(cmds, s.client.B().Sadd().Key(tag).Member(key).Build())

	for _, res := range s.client.DoMulti(ctx, cmds...) {
		if err := res.Error(); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
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
		return fmt.Errorf(
			"%s: refresh token not found: %w",
			op,
			storage.ErrRefreshTokenNotFound,
		)
	}

	return nil
}

func (s *Storage) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	return nil
}

func (s *Storage) RevokeAllAppTokens(ctx context.Context, appID int) error {
	const op = "storage.redis.RevokeAllAppTokens"

	tag := getAppTagKey(appID)
	var cursor uint64

	// remove tag members - tokens with app-tag
	for {
		resp := s.client.Do(
			ctx,
			s.client.B().
				Sscan().
				Key(tag).
				Cursor(cursor).
				Count(deleteAppTokenLimit).
				Build(),
		)
		if err := resp.Error(); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		entry, _ := resp.AsScanEntry()

		if len(entry.Elements) > 0 {
			err := s.client.Do(
				ctx,
				s.client.B().Del().Key(entry.Elements...).Build(),
			).Error()
			if err != nil {
				return fmt.Errorf("%s: %w", op, err)
			}
		}

		cursor = entry.Cursor
		if cursor == 0 {
			break
		}
	}

	// remove the tag index itself
	err := s.client.Do(ctx, s.client.B().Del().Key(tag).Build()).Error()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
