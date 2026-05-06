package jwt

import (
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
	UserID uuid.UUID `json:"sub"`
	Email  string    `json:"email"`
	AppID  uuid.UUID `json:"app_id"`
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
