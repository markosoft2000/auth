package jwt

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/markosoft2000/auth/internal/domain/models"
)

func GenerateToken(
	user *models.User,
	appID uuid.UUID,
	duration time.Duration,
	appSecret []byte,
) (string, error) {
	key, err := jwt.ParseEdPrivateKeyFromPEM(appSecret)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub":    user.ID,
		"email":  user.Email,
		"exp":    time.Now().Add(duration).Unix(),
		"app_id": appID,
		"iss":    "markosoft2000",
		"aud":    "auth-service",
	})

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

type CustomTokenClaims struct {
	jwt.RegisteredClaims
	Email  string    `json:"email"`
	UserID uuid.UUID `json:"sub"`
	AppID  uuid.UUID `json:"app_id"`
}

func GetClaimsUnverified(token string) (*CustomTokenClaims, error) {
	claims := &CustomTokenClaims{}

	_, _, err := new(jwt.Parser).ParseUnverified(token, claims)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// PublicKeyFromPrivatePEM derives the Ed25519 public key in PEM format from a private key PEM.
func PublicKeyFromPrivatePEM(privateKeyPEM []byte) (string, error) {
	// 1. Parse the private key from PEM
	key, err := jwt.ParseEdPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// 2. Assert type to ed25519.PrivateKey
	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not an ed25519 private key given")
	}

	// 3. Extract the public key and marshal to PKIX format
	pubKey := privKey.Public().(ed25519.PublicKey)
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	// 4. Encode to PEM format
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubKeyBytes}

	return string(pem.EncodeToMemory(block)), nil
}
