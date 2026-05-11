package models

import "github.com/google/uuid"

type User struct {
	Email    string
	PassHash string
	ID       uuid.UUID
}
