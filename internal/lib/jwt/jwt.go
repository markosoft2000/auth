package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markosoft2000/auth/internal/domain/models"
)

func GenerateToken(user models.User, app models.App, duration time.Duration) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(app.Secret))
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":    user.ID,
		"email":  user.Email,
		"exp":    time.Now().Add(duration).Unix(),
		"app_id": app.ID,
		"iss":    "markosoft2000",
		"aud":    "auth-service",
	})

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
