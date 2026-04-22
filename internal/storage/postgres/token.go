package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
)

func (s *Storage) SaveRefreshToken(
	ctx context.Context,
	token *models.RefreshToken,
) error {
	const op = "storage.postgres.SaveRefreshToken"

	query := "INSERT INTO refresh_tokens(user_id, app_id, token, expires_at, created_at, ip_address) VALUES($1, $2, $3, $4, $5, $6)"

	_, err := s.masterPool.Exec(
		ctx, query,
		token.UserID,
		token.AppID,
		token.Token,
		token.ExpiresAt,
		token.CreatedAt,
		token.IP_address,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) RefreshToken(
	ctx context.Context,
	token string,
	appID int,
) (*models.RefreshToken, error) {
	const op = "storage.postgres.RefreshToken"

	tokenModel := &models.RefreshToken{
		Token: token,
	}

	query := "SELECT user_id, app_id, expires_at, created_at, revoked, ip_address FROM refresh_tokens WHERE token = $1"

	err := s.replicaPool.QueryRow(ctx, query, token).Scan(
		&tokenModel.UserID,
		&tokenModel.AppID,
		&tokenModel.ExpiresAt,
		&tokenModel.CreatedAt,
		&tokenModel.Revoked,
		&tokenModel.IP_address,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, storage.ErrRefreshTokenNotFound)
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return tokenModel, nil
}

func (s *Storage) RevokeToken(ctx context.Context, token string, appID int) error {
	const op = "storage.postgres.RevokeToken"

	query := "UPDATE refresh_tokens SET revoked = TRUE WHERE token = $1"

	_, err := s.masterPool.Exec(ctx, query, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrRefreshTokenNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	const op = "storage.postgres.RevokeAllUserTokens"

	query := "UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 and revoked = FALSE"

	_, err := s.masterPool.Exec(ctx, query, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) RevokeAllAppTokens(ctx context.Context, appID int) error {
	const op = "storage.postgres.RevokeAllAppTokens"

	query := "UPDATE refresh_tokens SET revoked = TRUE WHERE app_id = $1 and revoked = FALSE"

	_, err := s.masterPool.Exec(ctx, query, appID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
