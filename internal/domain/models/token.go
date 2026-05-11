package models

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	IP_address netip.Addr `json:"ip_address"`
	Token      string     `json:"token"`
	UserID     uuid.UUID  `json:"user_id"`
	AppID      uuid.UUID  `json:"app_id"`
	Revoked    bool       `json:"revoked"`
}
