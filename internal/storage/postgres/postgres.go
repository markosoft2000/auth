package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New(host string, port int, user, password, dbname, sslmode string) (*Storage, error) {
	const op = "storage.postgres.New"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Storage{pool: pool}, nil
}

func (s *Storage) Stop() {
	s.pool.Close()
}

// user section

func (s *Storage) User(ctx context.Context, email string) (*models.User, error) {
	const op = "storage.postgres.User"

	user := &models.User{}
	query := "SELECT id, email, pass_hash FROM users WHERE email = $1"

	err := s.pool.QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &user.PassHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return &models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "storage.postgres.IsAdmin"

	var isAdmin bool
	query := "SELECT is_admin FROM admins WHERE id = $1"

	err := s.pool.QueryRow(ctx, query, userID).Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("%s: %w", op, err)
	}

	return isAdmin, nil
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash string) (uid int64, err error) {
	const op = "storage.postgres.SaveUser"

	var id int64
	query := "INSERT INTO users(email, pass_hash) VALUES($1, $2) RETURNING id"

	err = s.pool.QueryRow(ctx, query, email, passHash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// app section

func (s *Storage) App(ctx context.Context, appID int) (*models.App, error) {
	const op = "storage.postgres.App"

	app := &models.App{}
	query := "SELECT id, name, secret FROM apps WHERE id = $1"

	err := s.pool.QueryRow(ctx, query, appID).Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &models.App{}, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return &models.App{}, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

// token section

func (s *Storage) SaveRefreshToken(
	ctx context.Context,
	userID int64,
	token string,
	expiresAt time.Time,
	ip netip.Addr,
) error {
	const op = "storage.postgres.SaveRefreshToken"

	query := "INSERT INTO refresh_tokens(user_id, token, expires_at, ip_address) VALUES($1, $2, $3, $4)"

	_, err := s.pool.Exec(ctx, query, userID, token, expiresAt, ip)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) RefreshToken(
	ctx context.Context,
	token string,
) (*models.RefreshToken, error) {
	const op = "storage.postgres.RefreshToken"

	tokenModel := &models.RefreshToken{}

	query := "SELECT user_id, revoked, ip_address FROM refresh_tokens WHERE token = $1"

	err := s.pool.QueryRow(ctx, query, token).Scan(&tokenModel.UserID, &tokenModel.Revoked, &tokenModel.IP_address)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &models.RefreshToken{}, fmt.Errorf("%s: %w", op, storage.ErrRefreshTokenNotFound)
		}

		return &models.RefreshToken{}, fmt.Errorf("%s: %w", op, err)
	}

	return tokenModel, nil
}

func (s *Storage) RevokeToken(ctx context.Context, token string) error {
	const op = "storage.postgres.RevokeToken"

	query := "UPDATE refresh_tokens SET revoked = TRUE WHERE token = $1"

	_, err := s.pool.Exec(ctx, query, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrRefreshTokenNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) RevokeAllTokens(ctx context.Context, userId int64) error {
	const op = "storage.postgres.RevokeAllTokens"

	query := "UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 and revoked = FALSE"

	_, err := s.pool.Exec(ctx, query, userId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
