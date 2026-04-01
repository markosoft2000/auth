package suite

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/markosoft2000/auth/internal/config"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	grpcHost = "localhost"
)

type Suite struct {
	*testing.T
	Cfg        *config.Config
	AuthClient authv1.AuthClient
}

func New(t *testing.T) (context.Context, *Suite) {
	t.Helper()

	if os.Getenv("CONFIG_PATH") == "" {
		os.Setenv("CONFIG_PATH", "../configs/local_tests.yaml")
	}

	cfg := config.MustLoad()

	ctx, cancelCtx := context.WithTimeout(context.Background(), cfg.GRPC.Timeout)

	target := net.JoinHostPort(grpcHost, strconv.Itoa(cfg.GRPC.Port))

	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc client creation failed: %v", err)
	}

	t.Cleanup(func() {
		t.Helper()
		cancelCtx()
		cc.Close()
	})

	return ctx, &Suite{
		T:          t,
		Cfg:        cfg,
		AuthClient: authv1.NewAuthClient(cc),
	}
}
