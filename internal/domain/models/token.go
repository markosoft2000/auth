package models

import (
	"net/netip"
	"time"
)

type RefreshToken struct {
	UserID     int64
	AppID      int
	Token      string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	Revoked    bool
	IP_address netip.Addr
}
