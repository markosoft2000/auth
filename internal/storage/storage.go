package storage

import (
	"context"
	"errors"

	"github.com/markosoft2000/auth/internal/domain/models"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrAppNotFound  = errors.New("app not found")
)

type mockStorage struct{}

func NewMockStorage() *mockStorage {
	return &mockStorage{}
}

func (m *mockStorage) User(ctx context.Context, email string) (*models.User, error) {
	return &models.User{ID: 1, Email: "example@example.com", PassHash: ""}, nil
}

func (m *mockStorage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	return true, nil
}

func (m *mockStorage) SaveUser(ctx context.Context, email string, passHash string) (uid int64, err error) {
	return 1, nil
}

func (m *mockStorage) App(ctx context.Context, appID int) (*models.App, error) {
	return &models.App{ID: 1, Name: "App", Secret: "Secret"}, nil
}
