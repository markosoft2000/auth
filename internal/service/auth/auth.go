package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/markosoft2000/auth/internal/domain/models"
)

type Hasher interface {
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) bool
}

type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(encrypted []byte) ([]byte, error)
}

type PubSub interface {
	ProduceUserActivityEvent(key, data []byte) error
	ProduceAppKeyEvent(key, data []byte) error
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
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
}

// UserSaver handles User-related writes
type UserSaver interface {
	SaveUser(ctx context.Context, user *models.User) (uid uuid.UUID, err error)
}

// AppManager handles Application-related metadata (for JWT signing/secrets)
type AppManager interface {
	App(ctx context.Context, appID uuid.UUID) (*models.App, error)
	SaveApp(ctx context.Context, app *models.App) (id uuid.UUID, err error)
	DeleteApp(ctx context.Context, appID uuid.UUID) error
}

// TokenManager handles storing tokens
type TokenManager interface {
	RefreshToken(
		ctx context.Context,
		token string,
		userID uuid.UUID,
		appID uuid.UUID,
	) (*models.RefreshToken, error)

	SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error

	RevokeToken(ctx context.Context, token string, userID uuid.UUID, appID uuid.UUID) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
	RevokeAllAppTokens(ctx context.Context, appID uuid.UUID) error
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

type AuthCfg struct {
	TokenTTL               time.Duration
	RefreshTokenTTL        time.Duration
	ReissueRefreshTokenTTL time.Duration
}

type Auth struct {
	log                    *slog.Logger
	tokenTTL               time.Duration
	refreshTokenTTL        time.Duration
	reissueRefreshTokenTTL time.Duration
	hasher                 Hasher
	cipher                 Cipher
	userSaver              UserSaver
	userProvider           UserProvider
	appManager             AppManager
	tokenManager           TokenManager
	pubsub                 PubSub
}

func New(
	log *slog.Logger,
	cfg AuthCfg,
	hasher Hasher,
	cipher Cipher,
	storage Storage,
	pubsub PubSub,
) *Auth {
	return &Auth{
		log:                    log,
		tokenTTL:               cfg.TokenTTL,
		refreshTokenTTL:        cfg.RefreshTokenTTL,
		reissueRefreshTokenTTL: cfg.ReissueRefreshTokenTTL,
		hasher:                 hasher,
		cipher:                 cipher,
		userSaver:              storage.UserSaver,
		userProvider:           storage.UserProvider,
		appManager:             storage.AppManager,
		tokenManager:           storage.TokenManager,
		pubsub:                 pubsub,
	}
}
