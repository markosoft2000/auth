package authgrpc

import (
	"fmt"

	"github.com/google/uuid"
)

func convertStringToUUIDv7(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse UUID: %w", err)
	}

	if id.Version() != 7 {
		return uuid.Nil, fmt.Errorf("invalid UUID version (not v7): %s", s)
	}

	return id, nil
}
