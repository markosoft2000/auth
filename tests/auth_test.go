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
	appPublicKey = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAMYIOaERkcwrSychpWM3RxRYX1p3Et1ecDzEWNbQJpGo=
-----END PUBLIC KEY-----`
	appPrivateKey = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIDIX990wBqjSUJpPrqG9XNv270LxwwJW7kJurcTugllG
-----END PRIVATE KEY-----`
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

	appName := gofakeit.AppName()
	email := gofakeit.Email()
	password := genOkPassword()

	// 1. add app
	respAddApp, err := st.AuthClient.AddApp(ctx, &authv1.AddAppRequest{
		Name:   appName,
		Secret: []byte(appPrivateKey),
	})
	st.T.Logf("Login_HappyPath - app name: %s", appName)
	require.NoError(t, err)
	assert.NotEmpty(t, respAddApp.GetId())

	// 2. Register a user first
	respReg, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, respReg.GetUserId())

	// 3. Attempt Login
	respLogin, err := st.AuthClient.Login(ctx, &authv1.LoginRequest{
		Email:    email,
		Password: password,
		AppId:    respAddApp.GetId(),
		Ip:       "1.1.1.1",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, respLogin.GetAccessToken())
	assert.NotEmpty(t, respLogin.GetRefreshToken())

	// 4. Verify Token
	tokenClaims, err := validateToken(respLogin.GetAccessToken(), appPublicKey)
	require.NoError(t, err)
	claims := tokenClaims.(jwt.MapClaims)

	assert.Equal(t, respReg.GetUserId(), int64(claims["sub"].(float64)))
	assert.Equal(t, email, claims["email"].(string))
	assert.Equal(t, respAddApp.GetId(), int32(claims["app_id"].(float64)))

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

// TestAddDelApp_HappyPath adds and removes an app
func TestAddDelApp_HappyPath(t *testing.T) {
	t.Parallel()

	ctx, st := suite.New(t)

	appName := gofakeit.AppName()

	respAddApp, err := st.AuthClient.AddApp(ctx, &authv1.AddAppRequest{
		Name:   appName,
		Secret: []byte(appPrivateKey),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, respAddApp.GetId())

	_, err = st.AuthClient.RemoveApp(ctx, &authv1.RemoveAppRequest{
		Id: respAddApp.GetId(),
	})

	require.NoError(t, err)
}

func genOkPassword() string {
	return gofakeit.Password(true, true, true, true, false, 10) + "1!A_a"
}

func validateToken(tokenStr string, publicKeyPEM string) (jwt.Claims, error) {
	publicKey, err := jwt.ParseEdPublicKeyFromPEM([]byte(publicKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
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
