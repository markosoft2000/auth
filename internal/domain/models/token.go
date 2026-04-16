package models

import (
	"net/netip"
	"time"
)

type RefreshToken struct {
	UserID     int64      `json:"user_id"`
	AppID      int        `json:"app_id"`
	Token      string     `json:"token"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	Revoked    bool       `json:"revoked"`
	IP_address netip.Addr `json:"ip_address"`
}
