package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/lib/jwt"
	"github.com/markosoft2000/auth/internal/storage"
)

const (
	userActivityLogout        = "logout"
	userActivityLogin         = "login"
	userActivityLogoutAllApps = "logout_all_apps"
)

type userActivityEvent struct {
	EventType string    `json:"event_type"`
	Timestamp string    `json:"timestamp"`
	UserID    uuid.UUID `json:"user_id"`
	AppID     uuid.UUID `json:"app_id"`
}

// RegisterNewUser registers a new user
func (a *Auth) RegisterNewUser(
	ctx context.Context,
	email string,
	pass string,
) (uuid.UUID, error) {
	const op = "auth.RegisterNewUser"
	log := a.log.With(slog.String("op", op), slog.String("email", email))

	log.Info("registering new user")

	passHash, err := a.hasher.HashPassword(ctx, pass)
	if err != nil {
		log.Error("failed to hash password", slog.Any("error", err.Error()))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	userID, err := uuid.NewV7()
	if err != nil {
		log.Error("Failed to generate UUID", slog.Any("error", err))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userSaver.SaveUser(ctx, &models.User{
		ID:       userID,
		Email:    email,
		PassHash: passHash,
	})
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			log.Error("user exists", slog.Any("error", err))

			return uuid.Nil, fmt.Errorf("%s: %w", op, ErrUserExists)
		}

		log.Error("failed to save user", slog.Any("error", err))

		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// Login provides a new access and refresh tokens
func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appID uuid.UUID,
	ip string,
) (
	accessToken string,
	refreshToken string,
	err error,
) {
	const op = "auth.Login"
	log := a.log.With(slog.String("op", op), slog.String("email", email))

	log.Info("attempting login")

	app, err := a.appManager.App(ctx, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("app not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	user, err := a.userProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Error("user not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}

		log.Error("failed to get user", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	equal, err := a.hasher.ComparePassword(ctx, user.PassHash, password)
	if err != nil {
		log.Error("failed to compare hash password", slog.Any("error", err.Error()))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	if !equal {
		log.Warn("invalid credentials", slog.String("email", email))

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	decryptedAppSecret, err := a.cipher.Decrypt(app.Secret)
	if err != nil {
		log.Warn("invalid app key", slog.String("app_id", app.ID.String()))

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidAppKey)
	}

	accessToken, err = jwt.GenerateToken(user, app.ID, a.tokenTTL, decryptedAppSecret)
	if err != nil {
		log.Error("failed to generate accessToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	refreshToken, err = jwt.GenerateToken(user, app.ID, a.refreshTokenTTL, decryptedAppSecret)
	if err != nil {
		log.Error("failed to generate refreshToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	netIP, err := netip.ParseAddr(ip)
	if err != nil {
		return "", "", fmt.Errorf("invalid IP format - %s: %w", op, err)
	}

	// get&revoke old token
	oldRefreshToken, err := a.tokenManager.RefreshToken(ctx, "", user.ID, appID)
	if err != nil && !errors.Is(err, storage.ErrRefreshTokenNotFound) {
		log.Error("failed to get refresh token", slog.Any("error", err))
	} else if oldRefreshToken != nil {
		err = a.tokenManager.RevokeToken(ctx, oldRefreshToken.Token, user.ID, appID)
		if err != nil {
			log.Error("failed to revoke refresh token", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}
	}

	// save new token
	err = a.tokenManager.SaveRefreshToken(
		ctx,
		&models.RefreshToken{
			UserID:     user.ID,
			AppID:      app.ID,
			Token:      refreshToken,
			IP_address: netIP,
			CreatedAt:  time.Now(),
			ExpiresAt:  time.Now().Add(a.refreshTokenTTL),
		},
	)
	if err != nil && !errors.Is(err, storage.ErrRefreshTokenExits) {
		log.Error("failed to save refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in successfully")

	go a.sendUserActivityEvent(userActivityLogin, user.ID, app.ID)

	return accessToken, refreshToken, nil
}

func (a *Auth) Logout(
	ctx context.Context,
	userID uuid.UUID,
	appID uuid.UUID,
	allApp bool,
) error {
	const op = "auth.Logout"
	log := a.log.With(
		slog.String("op", op),
		slog.String("user_id", userID.String()),
		slog.String("app_id", appID.String()),
		slog.Bool("all_app", allApp),
	)

	log.Info("attempting logout")

	if allApp {
		err := a.tokenManager.RevokeAllUserTokens(ctx, userID)
		if err != nil {
			log.Error("failed to revoke all refresh tokens", slog.Any("error", err))

			return fmt.Errorf("%s: %w", op, err)
		}

		go a.sendUserActivityEvent(userActivityLogoutAllApps, userID, uuid.Nil)

		return nil
	}

	refreshToken, err := a.tokenManager.RefreshToken(ctx, "", userID, appID)
	if err != nil {
		if errors.Is(err, storage.ErrRefreshTokenNotFound) {
			log.Error("refresh token not found", slog.Any("error", err))

			return fmt.Errorf("%s: %w", op, ErrRefreshTokenNotFound)
		}

		log.Error("failed to get refresh token", slog.Any("error", err))

		return fmt.Errorf("%s: %w", op, err)
	}

	if refreshToken.Revoked {
		return nil
	}

	// revoking the token
	err = a.tokenManager.RevokeToken(ctx, refreshToken.Token, userID, appID)
	if err != nil {
		log.Error("failed to revoke refresh token", slog.Any("error", err))

		return fmt.Errorf("%s: %w", op, err)
	}

	go a.sendUserActivityEvent(userActivityLogout, userID, appID)

	return nil
}

// sendUserActivityEvent serializes and produces an auth event to the Kafka topic.
func (a *Auth) sendUserActivityEvent(eventType string, userID uuid.UUID, appID uuid.UUID) {
	op := "auth.sendUserActivityEvent"
	log := a.log.With(
		slog.String("op", op),
		slog.String("event_type", eventType),
		slog.String("user_id", userID.String()),
		slog.String("app_id", appID.String()),
	)

	event := userActivityEvent{
		EventType: eventType,
		UserID:    userID,
		AppID:     appID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Error("failed to marshal event", slog.Any("error", err))
		return
	}

	key := []byte(userID.String())
	if err := a.pubsub.ProduceUserActivityEvent(key, data); err != nil {
		log.Error("failed to push event to pubsub", slog.Any("error", err))
	}
}
