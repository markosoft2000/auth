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

type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(encrypted []byte) ([]byte, error)
}

type Storage struct {
	UserProvider UserProvider
	UserSaver    UserSaver
	AppManager   AppManager
	TokenManager TokenManager
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

// AppManager handles Application-related metadata (for JWT signing/secrets)
type AppManager interface {
	App(ctx context.Context, appID int) (*models.App, error)
	SaveApp(ctx context.Context, app *models.App) (id int, err error)
	DeleteApp(ctx context.Context, appID int) error
}

// TokenManager handles storing tokens
type TokenManager interface {
	RefreshToken(
		ctx context.Context,
		token string,
	) (*models.RefreshToken, error)

	SaveRefreshToken(
		ctx context.Context,
		userID int64,
		appId int,
		token string,
		expiresAt time.Time,
		ip netip.Addr,
	) error

	RevokeToken(ctx context.Context, token string) error

	RevokeAllUserTokens(ctx context.Context, userId int64) error
	RevokeAllAppTokens(ctx context.Context, appId int) error
}

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUserExists           = errors.New("user already exists")
	ErrUserNotFound         = errors.New("user not found")
	ErrAppNotFound          = errors.New("app not found")
	ErrAppExists            = errors.New("app already exists")
	ErrInvalidAppKey        = errors.New("invalid app key")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrInvalidIpAddress     = errors.New("invalid IP address for the session")
)

type Auth struct {
	log             *slog.Logger
	tokenTTL        time.Duration
	refreshTokenTTL time.Duration
	hasher          Hasher
	cipher          Cipher
	userSaver       UserSaver
	userProvider    UserProvider
	appManager      AppManager
	tokenManager    TokenManager
}

func New(
	log *slog.Logger,
	tokenTTL time.Duration,
	refreshTokenTTL time.Duration,
	hasher Hasher,
	cipher Cipher,
	storage Storage,
) *Auth {
	return &Auth{
		log:             log,
		tokenTTL:        tokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		hasher:          hasher,
		cipher:          cipher,
		userSaver:       storage.UserSaver,
		userProvider:    storage.UserProvider,
		appManager:      storage.AppManager,
		tokenManager:    storage.TokenManager,
	}
}

// RegisterNewUser registers a new user
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

// Login provides a new access and refresh tokens
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

	if !a.hasher.ComparePassword(user.PassHash, password) {
		log.Warn("invalid credentials", slog.String("email", email))

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	log.Info("user logged in successfully")

	decryptedAppSecret, err := a.cipher.Decrypt(app.Secret)
	if err != nil {
		log.Warn("invalid app key", slog.Int("app_id", app.ID))

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

	err = a.tokenManager.SaveRefreshToken(ctx, user.ID, app.ID, refreshToken, time.Now().Add(a.refreshTokenTTL), netIP)
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
		slog.Int64("user_id", user.ID),
		slog.Int("app_id", app.ID),
	)

	storedRefreshToken, err := a.tokenManager.RefreshToken(ctx, refreshToken)
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
		log.Warn("invalid app key", slog.Int("app_id", app.ID))

		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidAppKey)
	}

	newAccessToken, err = jwt.GenerateToken(user, app.ID, a.tokenTTL, decryptedAppSecret)
	if err != nil {
		log.Error("failed to generate accessToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// refresh token rotation
	newRefreshToken, err = jwt.GenerateToken(user, app.ID, a.refreshTokenTTL, decryptedAppSecret)
	if err != nil {
		log.Error("failed to generate refreshToken", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	err = a.tokenManager.SaveRefreshToken(
		ctx,
		user.ID,
		app.ID,
		newRefreshToken,
		time.Now().Add(a.refreshTokenTTL),
		storedRefreshToken.IP_address,
	)
	if err != nil {
		log.Error("failed to save refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	err = a.tokenManager.RevokeToken(ctx, refreshToken)
	if err != nil {
		log.Error("failed to revoke refresh token", slog.Any("error", err))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	return newAccessToken, newRefreshToken, nil
}

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
