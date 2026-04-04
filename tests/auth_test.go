package tests

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/golang-jwt/jwt/v5"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"
	"github.com/markosoft2000/auth/tests/suite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	emptyAppID   = 0
	appID        = 1
	appPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAlSJajhzgYWK7Ve/xLiAU
loq5+PeGTSZf+/YMXLH/caZeoGvWPFIcWuM8kW1W/USbqZx/O0sQnYCXPFof0Tu5
5sefNrXItqVgw61Qmvgzli5DjKHZIf339LElM3dOzrOKGFwqOI/WXs5TBcNQ5a8a
zV8WQE8j/+wZsMcqLqUALMM7l+JBAEGw3oy8RxEmLzhQ6EhpMxjGr9/ztJALBwbt
7BNjdxOz/kTO7++rgbz99fvQ+59PpR5ZmsmVS8yhHRZWszO+9qGbYW2X7gj/vWqL
1Tpg21vys/yV8L3dEcXgaLzQ7YfdQClWZD0M2AbgwHAoQh6vy6hGV0GYmw7n7HOL
JwIDAQAB
-----END PUBLIC KEY-----`
)

func TestRegister_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := genOkPassword()

	respReg, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: password,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, respReg.GetUserId())
}

func TestLogin_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := genOkPassword()

	// 1. Register a user first
	respReg, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, respReg.GetUserId())

	// 2. Attempt Login
	respLogin, err := st.AuthClient.Login(ctx, &authv1.LoginRequest{
		Email:    email,
		Password: password,
		AppId:    appID,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, respLogin.GetToken())

	// 3. Verify Token
	tokenClaims, err := validateToken(respLogin.GetToken(), appPublicKey)
	require.NoError(t, err)
	claims := tokenClaims.(jwt.MapClaims)

	assert.Equal(t, respReg.GetUserId(), int64(claims["sub"].(float64)))
	assert.Equal(t, email, claims["email"].(string))
	assert.Equal(t, appID, int(claims["app_id"].(float64)))

	expectedExpiration := time.Now().Add(st.Cfg.TokenTTL).Unix()
	const deltaSeconds = 1
	assert.InDelta(
		t,
		expectedExpiration,
		int64(claims["exp"].(float64)),
		deltaSeconds,
		fmt.Sprintf("Token expiration should be within %d seconds of expected time", deltaSeconds),
	)
}

func genOkPassword() string {
	return gofakeit.Password(true, true, true, true, false, 10) + "1!A_a"
}

func validateToken(tokenStr string, publicKeyPEM string) (jwt.Claims, error) {
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	},
		jwt.WithIssuer("markosoft2000"),
		jwt.WithAudience("auth-service"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token has expired")
		}
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return token.Claims, nil
}
