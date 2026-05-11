package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/lib/jwt"
	"github.com/markosoft2000/auth/internal/storage"
)

const (
	appKeyAddedEvent   = "app_key_added"
	appKeyRemovedEvent = "app_key_removed"
)

type appKeyEvent struct {
	EventType string    `json:"event_type"`
	PublicKey string    `json:"public_key"`
	AppID     uuid.UUID `json:"app_id"`
}

// AddApp adds a new app with a secret or private key
func (a *Auth) AddApp(ctx context.Context, appName string, appSecret []byte) (id uuid.UUID, err error) {
	const op = "auth.AddApp"
	log := a.log.With(slog.String("op", op), slog.String("app_name", appName))

	log.Info("adding new app")

	pubKey, err := jwt.PublicKeyFromPrivatePEM(appSecret)
	if err != nil {
		log.Error("failed to extract public key for the app", slog.Any("error", err))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	encryptedSecret, err := a.cipher.Encrypt(appSecret)
	if err != nil {
		log.Error("failed to encrypt secret", slog.Any("error", err.Error()))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	appID, err := uuid.NewV7()
	if err != nil {
		log.Error("Failed to generate UUID", slog.Any("error", err))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	app := &models.App{
		ID:     appID,
		Name:   appName,
		Secret: encryptedSecret,
	}

	id, err = a.appManager.SaveApp(ctx, app)
	if err != nil {
		if errors.Is(err, storage.ErrAppExists) {
			log.Error("app already exists", slog.Any("error", err))

			return uuid.Nil, fmt.Errorf("%s: %w", op, ErrAppExists)
		}

		log.Error("failed to save app", slog.Any("error", err))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	a.sendAppKeyEvent(appKeyAddedEvent, app.ID, pubKey)

	return id, nil
}

// RemoveApp deletes app
func (a *Auth) RemoveApp(ctx context.Context, appID uuid.UUID) error {
	const op = "auth.RemoveApp"
	log := a.log.With(slog.String("op", op), slog.String("app_id", appID.String()))

	log.Info("removing app")

	err := a.appManager.DeleteApp(ctx, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("app not found", slog.Any("error", err))

			return fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}

		log.Error("failed to remove app", slog.Any("error", err))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Warn("Revoking all refresh tokens for APP ID", slog.String("app_id", appID.String()))

	a.sendAppKeyEvent(appKeyRemovedEvent, appID, "")

	// clean up all refresh tokens associated with that app_id to ensure no active sessions remain
	err = a.tokenManager.RevokeAllAppTokens(ctx, appID)
	if err != nil {
		log.Error("failed to revoke all refresh token for the app", slog.Any("error", err))

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// sendAppKeyEvent serializes and produces an 'app added public key' event to the Kafka topic.
func (a *Auth) sendAppKeyEvent(eventType string, appID uuid.UUID, publicKey string) {
	op := "auth.sendAppKeyEvent"
	log := a.log.With(
		slog.String("op", op),
		slog.String("app_id", appID.String()),
	)

	event := appKeyEvent{
		EventType: eventType,
		AppID:     appID,
		PublicKey: publicKey,
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Error("failed to marshal event", slog.Any("error", err))
		return
	}

	key := []byte(appID.String())
	if err := a.pubsub.ProduceAppKeyEvent(key, data); err != nil {
		log.Error("failed to push event to pubsub", slog.Any("error", err))
	}
}
