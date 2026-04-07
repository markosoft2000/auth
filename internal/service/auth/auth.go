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

type Hasher interface {
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) bool
}

type Storage struct {
	UserProvider  UserProvider
	UserSaver     UserSaver
	AppProvider   AppProvider
	TokenProvider TokenProvider
}

// UserProvider handles User-related reads
type UserProvider interface {
	User(ctx context.Context, email string) (*models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

// UserSaver handles User-related writes
type UserSaver interface {
	SaveUser(
		ctx context.Context,
		email string,
		passHash string,
	) (uid int64, err error)
}

// AppProvider handles Application-related metadata (for JWT signing/secrets)
type AppProvider interface {
	App(ctx context.Context, appID int) (*models.App, error)
}

// TokenProvider handles storing tokens
type TokenProvider interface {
	RefreshToken(
		ctx context.Context,
		token string,
	) (*models.RefreshToken, error)

	SaveRefreshToken(
		ctx context.Context,
		userID int64,
		token string,
		expiresAt time.Time,
		ip netip.Addr,
	) error

	RevokeToken(ctx context.Context, token string) error

	RevokeAllTokens(ctx context.Context, userId int64) error
}

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUserExists           = errors.New("user already exists")
	ErrUserNotFound         = errors.New("user not found")
	ErrAppNotFound          = errors.New("app not found")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrInvalidIpAddress     = errors.New("invalid IP address for the session")
)

type Auth struct {
	log             *slog.Logger
	tokenTTL        time.Duration
	refreshTokenTTL time.Duration
	hasher          Hasher
	masterSecret    string
	userSaver       UserSaver
	userProvider    UserProvider
	appProvider     AppProvider
	tokenProvider   TokenProvider
}

func New(
	log *slog.Logger,
	tokenTTL time.Duration,
	refreshTokenTTL time.Duration,
	hasher Hasher,
	masterSecret string,
	storage Storage,
) *Auth {
	return &Auth{
		log:             log,
		tokenTTL:        tokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		hasher:          hasher,
		masterSecret:    masterSecret,
		userSaver:       storage.UserSaver,
		userProvider:    storage.UserProvider,
		appProvider:     storage.AppProvider,
		tokenProvider:   storage.TokenProvider,
	}
}

func (a *Auth) RegisterNewUser(
	ctx context.Context,
	email string,
	pass string,
) (int64, error) {
	const op = "auth.RegisterNewUser"
	log := a.log.With(slog.String("op", op), slog.String("email", email))

	log.Info("registering new user")

	passHash, err := a.hasher.HashPassword(pass)
	if err != nil {
		log.Error("failed to hash password", slog.Any("error", err.Error()))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			log.Error("user exists", slog.Any("error", err))

			return 0, fmt.Errorf("%s: %w", op, ErrUserExists)
		}

		log.Error("failed to save user", slog.Any("error", err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appID int,
	ip string,
) (
	accessToken string,
	refreshToken string,
	err error,
) {
	const op = "auth.Login"
	log := a.log.With(slog.String("op", op), slog.String("email", email))

	log.Info("attempting login")

	user, err := a.userProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Error("user not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}

		log.Error("failed to get user", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if !a.hasher.ComparePassword(user.PassHash, password) {
		log.Warn("invalid credentials", slog.String("email", email))

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("app not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in successfully")

	accessToken, err = jwt.GenerateToken(user, app.ID, a.tokenTTL, app.Secret, a.masterSecret)
	if err != nil {
		log.Error("failed to generate accessToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	refreshToken, err = jwt.GenerateToken(user, app.ID, a.refreshTokenTTL, app.Secret, a.masterSecret)
	if err != nil {
		log.Error("failed to generate refreshToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	netIP, err := netip.ParseAddr(ip)
	if err != nil {
		return "", "", fmt.Errorf("invalid IP format - %s: %w", op, err)
	}

	err = a.tokenProvider.SaveRefreshToken(ctx, user.ID, refreshToken, time.Now().Add(a.refreshTokenTTL), netIP)
	if err != nil {
		log.Error("failed to save refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	return accessToken, refreshToken, nil
}

func (a *Auth) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "auth.IsAdmin"

	log := a.log.With(slog.String("op", op), slog.Int64("user_id", userID))

	log.Info("role check - is admin")

	isAdmin, err := a.userProvider.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("failed to check admin status", slog.Any("error", err))

			return false, fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}

		return false, fmt.Errorf("%s: %w", op, err)
	}

	return isAdmin, nil
}

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

	app, err := a.appProvider.App(ctx, claims.AppID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("app not found", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	log = a.log.With(
		slog.String("op", op),
		slog.Int64("user_id", user.ID),
		slog.Int("app_id", app.ID),
	)

	storedRefreshToken, err := a.tokenProvider.RefreshToken(ctx, refreshToken)
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
		log.Warn("token revoked")

		return "", "", fmt.Errorf("%s: %w", op, ErrRefreshTokenNotFound)
	}

	// - if ip and stored ip are diff - > revoke ALL refresh tokens for the user
	netIP, err := netip.ParseAddr(ip)
	if err != nil {
		return "", "", fmt.Errorf("invalid IP format - %s: %w", op, err)
	}

	if storedRefreshToken.IP_address != netIP {
		log.Warn("invalid IP address for the session. token revoked.", slog.String("ip", ip))

		err := a.tokenProvider.RevokeAllTokens(ctx, user.ID)
		if err != nil {
			log.Error("failed to revoke refresh token", slog.Any("error", err))

			return "", "", fmt.Errorf("%s: %w", op, err)
		}

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidIpAddress)
	}

	// OK
	newAccessToken, err = jwt.GenerateToken(user, app.ID, a.tokenTTL, app.Secret, a.masterSecret)
	if err != nil {
		log.Error("failed to generate accessToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// refresh token rotation
	newRefreshToken, err = jwt.GenerateToken(user, app.ID, a.refreshTokenTTL, app.Secret, a.masterSecret)
	if err != nil {
		log.Error("failed to generate refreshToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	err = a.tokenProvider.SaveRefreshToken(
		ctx,
		user.ID,
		newRefreshToken,
		time.Now().Add(a.refreshTokenTTL),
		storedRefreshToken.IP_address,
	)
	if err != nil {
		log.Error("failed to save refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	err = a.tokenProvider.RevokeToken(ctx, refreshToken)
	if err != nil {
		log.Error("failed to revoke refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	return newAccessToken, newRefreshToken, nil
}
