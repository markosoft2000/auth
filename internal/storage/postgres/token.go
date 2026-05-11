package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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
	userID uuid.UUID,
	appID uuid.UUID,
) (*models.RefreshToken, error) {
	const op = "storage.postgres.RefreshToken"

	tokenModel := &models.RefreshToken{
		Token: token,
	}

	query := "SELECT user_id, app_id, token, expires_at, created_at, revoked, ip_address FROM refresh_tokens WHERE user_id = $1 AND app_id = $2"
	args := []any{userID, appID}

	if token != "" {
		query += " AND token = $3"
		args = append(args, token)
	}

	query += " ORDER BY created_at DESC LIMIT 1"

	err := s.replicaPool.QueryRow(ctx, query, args...).Scan(
		&tokenModel.UserID,
		&tokenModel.AppID,
		&tokenModel.Token,
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

func (s *Storage) RevokeToken(
	ctx context.Context,
	token string,
	userID uuid.UUID,
	appID uuid.UUID,
) error {
	const op = "storage.postgres.RevokeToken"

	query := "UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND app_id = $2"
	args := []any{userID, appID}

	if token != "" {
		query += " AND token = $3"
		args = append(args, token)
	}

	tag, err := s.masterPool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrRefreshTokenNotFound)
	}

	return nil
}

func (s *Storage) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
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

func (s *Storage) RevokeAllAppTokens(ctx context.Context, appID uuid.UUID) error {
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
