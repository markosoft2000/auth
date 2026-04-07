package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markosoft2000/auth/internal/domain/models"
	cipher "github.com/markosoft2000/auth/internal/lib/crypt"
)

func GenerateToken(
	user *models.User,
	appID int,
	duration time.Duration,
	appSecret []byte,
	masterSecret string,
) (string, error) {
	decryptedPEM, err := cipher.DecryptKey(appSecret, masterSecret)
	if err != nil {
		return "", err
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(decryptedPEM)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
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
	UserID int64  `json:"sub"`
	Email  string `json:"email"`
	AppID  int    `json:"app_id"`
	jwt.RegisteredClaims
}

func GetClaimsUnverified(token string) (*CustomTokenClaims, error) {
	claims := &CustomTokenClaims{}

	_, _, err := new(jwt.Parser).ParseUnverified(token, claims)
	if err != nil {
		return nil, err
	}

	return claims, nil
}
