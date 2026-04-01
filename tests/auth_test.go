package tests

import (
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
	emptyAppID = 0
	appID      = 1
	appSecret  = "testSecret"
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

	// 3. Verify Token
	require.NoError(t, err)
	assert.NotEmpty(t, respLogin.GetToken())

	// 4. Token Validation
	tokenParsed, err := jwt.Parse(respLogin.GetToken(), func(token *jwt.Token) (interface{}, error) {
		return []byte(appSecret), nil
	})
	require.NoError(t, err)

	claims, ok := tokenParsed.Claims.(jwt.MapClaims)
	require.True(t, ok)

	assert.Equal(t, respReg.GetUserId(), int64(claims["uid"].(float64)))
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
