package models

import "github.com/google/uuid"

type App struct {
	Name   string
	Secret []byte
	ID     uuid.UUID
}
