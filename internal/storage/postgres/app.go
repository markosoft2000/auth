package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
)

func (s *Storage) App(ctx context.Context, appID int) (*models.App, error) {
	const op = "storage.postgres.App"

	app := &models.App{}
	query := "SELECT id, name, secret FROM apps WHERE id = $1"

	err := s.pool.QueryRow(ctx, query, appID).Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

func (s *Storage) SaveApp(ctx context.Context, app *models.App) (id int, err error) {
	const op = "storage.postgres.SaveApp"

	query := "INSERT INTO apps(name, secret) VALUES($1, $2) RETURNING id"

	err = s.pool.QueryRow(ctx, query, app.Name, app.Secret).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrAppExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) DeleteApp(ctx context.Context, appID int) error {
	const op = "storage.postgres.DeleteApp"

	query := "DELETE FROM apps WHERE id = $1"

	_, err := s.pool.Exec(ctx, query, appID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
