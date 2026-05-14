package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/lib/jwt"
	"github.com/markosoft2000/auth/internal/storage"
)

// RefreshToken provides a new access token and refresh token with rotation
func (a *Auth) RefreshToken(
	ctx context.Context,
	refreshToken string,
	ip string,
) (
	newAccessToken string,
	newRefreshToken string,
	err error,
) {
	const op = "auth.RefreshToken"

	log := a.log.With(slog.String("op", op))
	log.Info("refreshing access token")

	claims, err := jwt.GetClaimsUnverified(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	user := &models.User{
		ID:    claims.UserID,
		Email: claims.Email,
	}

	app, err := a.appManager.App(ctx, claims.AppID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("app not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	log = a.log.With(
		slog.String("op", op),
		slog.String("user_id", user.ID.String()),
		slog.String("app_id", app.ID.String()),
	)

	/*
		if caching is used for refresh tokens and an app has been deleted
		no need to invalidate token cache intentionally because
		we can't retrieve a refresh token without the app (call above in this method)
		meanwhile there are access tokens still remain active for some period of time,
		user permissions will be checked during actions which include "is app active"
		(it's expected and planed that some functional has been working for a bit longer)

		Only 'revoke token' operation should delete the related cached token
	*/
	storedRefreshToken, err := a.tokenManager.RefreshToken(ctx, refreshToken, user.ID, app.ID)
	if err != nil {
		if errors.Is(err, storage.ErrRefreshTokenNotFound) {
			log.Error("refresh token not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrRefreshTokenNotFound)
		}

		log.Error("failed to get refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// security section
	// if the token revoked
	if storedRefreshToken.Revoked {
		log.Warn("revoked token was used. revoking all user tokens for all app due to a possible attack")

		err := a.tokenManager.RevokeAllUserTokens(ctx, user.ID)
		if err != nil {
			log.Error("failed to revoke refresh token", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}

		return "", "", fmt.Errorf("%s: %w", op, ErrRefreshTokenNotFound)
	}

	// - if ip and stored ip are diff - > revoke ALL refresh tokens for the user
	netIP, err := netip.ParseAddr(ip)
	if err != nil {
		return "", "", fmt.Errorf("invalid IP format - %s: %w", op, err)
	}

	if storedRefreshToken.IP_address != netIP {
		log.Warn("invalid IP address for the session. token revoked.", slog.String("ip", ip))

		err := a.tokenManager.RevokeAllUserTokens(ctx, user.ID)
		if err != nil {
			log.Error("failed to revoke refresh token", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidIpAddress)
	}

	// OK
	decryptedAppSecret, err := a.cipher.Decrypt(app.Secret)
	if err != nil {
		log.Warn("invalid app key", slog.String("app_id", app.ID.String()))

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidAppKey)
	}

	newAccessToken, err = jwt.GenerateToken(user, app.ID, a.tokenTTL, decryptedAppSecret)
	if err != nil {
		log.Error("failed to generate accessToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// refresh token rotation
	if time.Since(storedRefreshToken.CreatedAt) > a.reissueRefreshTokenTTL {
		newRefreshToken, err = jwt.GenerateToken(user, app.ID, a.refreshTokenTTL, decryptedAppSecret)
		if err != nil {
			log.Error("failed to generate refreshToken", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}

		err = a.tokenManager.SaveRefreshToken(
			ctx,
			&models.RefreshToken{
				UserID:     user.ID,
				AppID:      app.ID,
				Token:      newRefreshToken,
				IP_address: storedRefreshToken.IP_address,
				CreatedAt:  storedRefreshToken.CreatedAt,
				ExpiresAt:  time.Now().Add(a.refreshTokenTTL),
			},
		)
		if err != nil && !errors.Is(err, storage.ErrRefreshTokenExits) {
			log.Error("failed to save refresh token", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}

		err = a.tokenManager.RevokeToken(ctx, refreshToken, user.ID, app.ID)
		if err != nil {
			log.Error("failed to revoke refresh token", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}

		return newAccessToken, newRefreshToken, nil
	}

	return newAccessToken, "", nil
}
