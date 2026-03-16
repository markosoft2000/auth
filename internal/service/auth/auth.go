package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
)

type Hasher interface {
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) bool
}

// UserProvider handles User-related reads
type UserProvider interface {
	User(ctx context.Context, email string) (*models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

// UserSaver handles User-related writes
type UserSaver interface {
	SaveUser(ctx context.Context, email string, passHash string) (uid int64, err error)
}

// AppProvider handles Application-related metadata (for JWT signing/secrets)
type AppProvider interface {
	App(ctx context.Context, appID int) (*models.App, error)
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type Auth struct {
	log          *slog.Logger
	tokenTTL     time.Duration
	hasher       Hasher
	userSaver    UserSaver
	userProvider UserProvider
	appProvider  AppProvider
}

func New(
	log *slog.Logger,
	tokenTTL time.Duration,
	hasher Hasher,
	userSaver UserSaver,
	userProvider UserProvider,
	appProvider AppProvider,
) *Auth {
	return &Auth{
		log:          log,
		tokenTTL:     tokenTTL,
		hasher:       hasher,
		userSaver:    userSaver,
		userProvider: userProvider,
		appProvider:  appProvider,
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
) (string, error) {
	const op = "auth.Login"
	log := a.log.With(slog.String("op", op), slog.String("email", email))

	log.Info("attempting login")

	user, err := a.userProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return "", ErrInvalidCredentials
		}

		log.Error("failed to get user", slog.Any("error", err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	if !a.hasher.ComparePassword(user.PassHash, password) {
		log.Warn("invalid credentials", slog.String("email", email))

		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in successfully")

	// TODO: Generate JWT using app.Secret and user.ID
	token := "token-for-app-" + app.Name

	return token, nil
}

func (a *Auth) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "auth.IsAdmin"

	log := a.log.With(slog.String("op", op), slog.Int64("user_id", userID))

	log.Info("role check - is admin")

	isAdmin, err := a.userProvider.IsAdmin(ctx, userID)
	if err != nil {
		log.Error("failed to check admin status", slog.Any("error", err))

		return false, fmt.Errorf("%s: %w", op, err)
	}

	return isAdmin, nil
}
