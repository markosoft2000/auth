package postgres

import (
	"context"
	"fmt"

	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markosoft2000/auth/internal/domain/models"
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

func (s *Storage) User(ctx context.Context, email string) (*models.User, error) {
	return &models.User{ID: 1, Email: "example@example.com", PassHash: ""}, nil
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	return true, nil
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash string) (uid int64, err error) {
	return 1, nil
}

func (s *Storage) App(ctx context.Context, appID int) (*models.App, error) {
	return &models.App{ID: 1, Name: "App", Secret: "Secret"}, nil
}
