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
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAlSJajhzgYWK7Ve/xLiAU
loq5+PeGTSZf+/YMXLH/caZeoGvWPFIcWuM8kW1W/USbqZx/O0sQnYCXPFof0Tu5
5sefNrXItqVgw61Qmvgzli5DjKHZIf339LElM3dOzrOKGFwqOI/WXs5TBcNQ5a8a
zV8WQE8j/+wZsMcqLqUALMM7l+JBAEGw3oy8RxEmLzhQ6EhpMxjGr9/ztJALBwbt
7BNjdxOz/kTO7++rgbz99fvQ+59PpR5ZmsmVS8yhHRZWszO+9qGbYW2X7gj/vWqL
1Tpg21vys/yV8L3dEcXgaLzQ7YfdQClWZD0M2AbgwHAoQh6vy6hGV0GYmw7n7HOL
JwIDAQAB
-----END PUBLIC KEY-----`
	appPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCVIlqOHOBhYrtV
7/EuIBSWirn494ZNJl/79gxcsf9xpl6ga9Y8Uhxa4zyRbVb9RJupnH87SxCdgJc8
Wh/RO7nmx582tci2pWDDrVCa+DOWLkOModkh/ff0sSUzd07Os4oYXCo4j9ZezlMF
w1DlrxrNXxZATyP/7BmwxyoupQAswzuX4kEAQbDejLxHESYvOFDoSGkzGMav3/O0
kAsHBu3sE2N3E7P+RM7v76uBvP31+9D7n0+lHlmayZVLzKEdFlazM772oZthbZfu
CP+9aovVOmDbW/Kz/JXwvd0RxeBovNDth91AKVZkPQzYBuDAcChCHq/LqEZXQZib
Dufsc4snAgMBAAECggEAN5qGbuQfWNAKDwGWR99YLcTtkAEiBtPfB792Xn1TQ5mH
QILUbssinVSKOaKtw2J4M2YRHhdfy4mtWag0oruFFaGqICjo/AhvKqYNHqqZ+U2o
i9+waHcboxUFTMoYDQNt4qVCbLJceB6vX7+yAuEluhrQ3gtj3cC21216scd9aDKU
1J99ig5iMlOQAFFPwfrX1aKol5ts/orVIoZj5TiVraB6+fajcmyMpworQSzP16g/
Hr4hIl+hpucHUyRJgkX4vff2hzteXL+aPrhipBdho7YX5ErkiB3Vxc2hvyJmd8rN
FoZNAwOMiUl57uIpF2xgPcqxoych+LPtKo8Rd+HeOQKBgQDKLfvtrX6q5GzM+iLy
pAVnisk8/oZfhnS5Br008Ngtzka/GCIpi+ma0snxnIM5yNhhCZk9VYKpJEIpaQBw
OtNBzIWAhXuQHF/N/t/DV2x+IlV4UGWXWEH72QEwEj72oB/dnBMXlI/8BnjdVZhH
ZK8QkWWa50q4u+GYuEZ18YNROwKBgQC81XYeFcb9zDfRgX06v6u1819SJkKrHU89
GGtU8YpRitRbbUpOZKcWB2OT9JV61X/QN3Ngg8uhecJZ1SkyawVU1Lys+vttwrvO
4DmeKaVcJd59gg8wTHXdbkVcSZtTfgau7tAv1rHSK3+nmdtr7Mzh5psPp7/NlHDA
149WB1mPBQKBgFU2iGYmp6qTWCAUlUI7S2PWpPamODBu1Sde5cQ4doTn2f2UyGFG
bREqIp9I3i4urrRHfWTSc52igJg/f0XOJVgoQWRn3iphKygBcoI8iKepBOkOyaK+
OiFR1yRRrGP6HTQkIg/gN8d7Wtm+x83fa8HJ5k8hiObPmUfq8xem0TgdAoGAYl2j
4PohJXYrIYSdkmvj660yW1243uAutbmxt5b3IZD7HAErcvi1nSEOOzVuZIUwxmsM
PBuLiLsfhaIniq77IPyMqGM5dCy7noFpIj25eO31H1YPyW4a+9UEZpWlRvHgU2Ht
qu3gxYWJQuo/xdGdzJNO9PHCVTndHmdrsDm16m0CgYAuHhW0RpDfWr5nhsguit7G
hSorOTvmEnLxSIAWM8T5h+z0mC9LBumRkQERy/jZO3M2IKadsAgaYd8fRqFcZVqN
konGTjP3r/H86xIK6qnr7ZMYpwkt+82rB20c7gZaZ+mKogGv+2OCw0owGUQ1qpJo
jGMLd/q/Kml/KQdIo/ml/Q==
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
