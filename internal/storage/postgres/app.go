package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
)

func (s *Storage) App(ctx context.Context, appID uuid.UUID) (*models.App, error) {
	const op = "storage.postgres.App"

	app := &models.App{}
	query := "SELECT id, name, secret FROM apps WHERE id = $1"

	err := s.replicaPool.QueryRow(ctx, query, appID).Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

func (s *Storage) SaveApp(ctx context.Context, app *models.App) (id uuid.UUID, err error) {
	const op = "storage.postgres.SaveApp"

	query := "INSERT INTO apps(id, name, secret) VALUES($1, $2, $3) RETURNING id"

	err = s.masterPool.QueryRow(ctx, query, app.ID, app.Name, app.Secret).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		// if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return uuid.Nil, fmt.Errorf("%s: %w", op, storage.ErrAppExists)
		}

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) DeleteApp(ctx context.Context, appID uuid.UUID) error {
	const op = "storage.postgres.DeleteApp"

	query := "DELETE FROM apps WHERE id = $1"

	tag, err := s.masterPool.Exec(ctx, query, appID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
	}

	return nil
}
