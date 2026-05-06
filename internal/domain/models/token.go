package models

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	UserID     uuid.UUID  `json:"user_id"`
	AppID      uuid.UUID  `json:"app_id"`
	Token      string     `json:"token"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	Revoked    bool       `json:"revoked"`
	IP_address netip.Addr `json:"ip_address"`
}
