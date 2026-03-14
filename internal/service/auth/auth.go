package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type AuthConfig struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
	TokenTTL    time.Duration
}

type Auth struct {
	log *slog.Logger
	cfg AuthConfig
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)

func New(
	log *slog.Logger,
	cfg AuthConfig,
) *Auth {
	return &Auth{
		log: log,
		cfg: cfg,
	}
}

func (a *Auth) RegisterNewUser(
	ctx context.Context,
	email string,
	pass string,
) (int64, error) {
	return 1, nil
}

func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appID int,
) (string, error) {
	return "token", nil
}

func (a *Auth) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	return true, nil
}
