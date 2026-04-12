package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
)

// AddApp adds a new app with a secret or private key
func (a *Auth) AddApp(ctx context.Context, appName string, appSecret []byte) (id int, err error) {
	const op = "auth.AddApp"
	log := a.log.With(slog.String("op", op), slog.String("app_name", appName))

	log.Info("adding new app")

	encryptedSecret, err := a.cipher.Encrypt(appSecret)
	if err != nil {
		log.Error("failed to encrypt secret", slog.Any("error", err.Error()))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	app := &models.App{
		Name:   appName,
		Secret: encryptedSecret,
	}

	id, err = a.appManager.SaveApp(ctx, app)
	if err != nil {
		if errors.Is(err, storage.ErrAppExists) {
			log.Error("app already exists", slog.Any("error", err))

			return 0, fmt.Errorf("%s: %w", op, ErrAppExists)
		}

		log.Error("failed to save app", slog.Any("error", err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// RemoveApp deletes app
func (a *Auth) RemoveApp(ctx context.Context, appId int) error {
	const op = "auth.RemoveApp"
	log := a.log.With(slog.String("op", op), slog.Int("app_id", appId))

	log.Info("removing app")

	err := a.appManager.DeleteApp(ctx, appId)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("app not found", slog.Any("error", err))

			return fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}

		log.Error("failed to remove app", slog.Any("error", err))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Warn("Revoking all refresh tokens for APP ID", slog.Int("app_id", appId))

	// clean up all refresh tokens associated with that app_id to ensure no active sessions remain
	err = a.tokenManager.RevokeAllAppTokens(ctx, appId)
	if err != nil {
		log.Error("failed to revoke all refresh token for the app", slog.Any("error", err))

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
