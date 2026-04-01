package tests

import (
	"testing"

	"github.com/brianvoe/gofakeit/v6"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"
	"github.com/markosoft2000/auth/tests/suite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegister_DuplicateEmail(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := genOkPassword()

	// 1. Register first time
	_, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)

	// 2. Register with SAME email
	respReg, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: password,
	})

	// 3. Assertions: Should return error and no user ID
	st.T.Logf("Received error: %v", err)
	require.Error(t, err)
	assert.Empty(t, respReg.GetUserId())
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestRegister_EmptyPassword(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()

	// Execute with empty password
	_, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: "",
	})

	// Assertions: Validation should catch this
	st.T.Logf("Received error: %v", err)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	assert.Contains(t, err.Error(), "password")
	assert.Contains(t, err.Error(), "validation errors")
}

func TestLogin_InvalidCredentials(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := genOkPassword()

	// 1. Register first time
	_, err := st.AuthClient.Register(ctx, &authv1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)

	// 2. Login with wrong password
	respLogin, err := st.AuthClient.Login(ctx, &authv1.LoginRequest{
		Email:    email,
		Password: "wrongPassword",
		AppId:    1,
	})

	// 3. Assertions
	st.T.Logf("Received error: %v", err)
	require.Error(t, err)
	assert.Empty(t, respLogin.GetToken())

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
