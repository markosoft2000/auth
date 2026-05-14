package main

import (
	"context"
	"fmt"
	"time"

	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"
	"github.com/markosoft2000/auth/tests/load"
	"github.com/spf13/afero"
	"github.com/yandex/pandora/cli"
	"github.com/yandex/pandora/core"
	"github.com/yandex/pandora/core/aggregator/netsample"
	coreimport "github.com/yandex/pandora/core/import"
	"github.com/yandex/pandora/core/register"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Ammo defines the structure of your test data
type Ammo struct {
	ID           string `json:"id"`
	IP           string `json:"ip"`
	RefreshToken string `json:"refresh_token"`
}

// GunConfig defines configurations passed via load.yaml
type GunConfig struct {
	Target string `yaml:"target"`
}

// Gun implements Pandora's core load-generation lifecycle
type Gun struct {
	core.Gun
	conf   GunConfig
	client authv1.AuthClient
	conn   *grpc.ClientConn
	aggr   core.Aggregator
	tag    string
}

// NewGun instantiates a custom gRPC load generator
func NewGun(conf GunConfig) *Gun {
	return &Gun{
		conf: conf,
		tag:  "login_test",
	}
}

// Bind configures dependencies and dials the target server
func (g *Gun) Bind(aggr core.Aggregator, deps core.GunDeps) error {
	g.aggr = aggr

	// Pull target directly from GunConfig resolved via YAML file
	target := g.conf.Target
	if target == "" {
		target = "localhost:50001"
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", target, err)
	}

	g.conn = conn
	g.client = authv1.NewAuthClient(conn)
	return nil
}

// Shoot processes a single transaction step
func (g *Gun) Shoot(ammo core.Ammo) {
	customAmmo := ammo.(*Ammo)

	// Instantiate net sample wrapper for tracking stats
	sample := netsample.Acquire(g.tag)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := &authv1.RefreshTokenRequest{
		Ip:           customAmmo.IP,
		RefreshToken: customAmmo.RefreshToken,
	}

	// Track start time
	startTime := time.Now()
	_, err := g.client.RefreshToken(ctx, req)

	// Record performance metric parameters
	sample.SetLatency(time.Since(startTime))
	if err != nil {
		// Extract the specific cryptographic code payload out of the gRPC layer
		st, ok := status.FromError(err)
		if ok {
			// SetUserNet maps an integer status cleanly directly into Column 7
			sample.SetUserNet(int(st.Code()))

			switch st.Code() {
			case codes.DeadlineExceeded, codes.Canceled:
				// Map network execution drops explicitly to internal code 999 in Column 11
				sample.SetProtoCode(999)
			case codes.ResourceExhausted:
				// Explicit tracking validation marker for your rate limiter triggers in Column 11
				sample.SetProtoCode(888)
			default:
				sample.SetProtoCode(1)
			}
		} else {
			// Catch low-level networking/socket drops
			sample.SetUserNet(0)
			sample.SetProtoCode(999)
		}
		sample.SetErr(err)
	} else {
		// Success status code configuration mapping (gRPC OK = 0)
		sample.SetUserNet(0)
		sample.SetProtoCode(0)
	}

	// Report findings back to the engine aggregator
	g.aggr.Report(sample)
}

// Close cleans up connections when the pool shuts down
func (g *Gun) Close() {
	if g.conn != nil {
		g.conn.Close()
	}
}

// go run main.go load.yaml
func main() {
	fs := afero.NewOsFs()
	coreimport.Import(fs) // Initialise standard framework asset loaders

	// Correctly register the custom provider via coreimport
	coreimport.RegisterCustomJSONProvider("json_ammo", func() core.Ammo {
		return &Ammo{}
	})

	// Register your Custom Gun Type
	register.Gun("grpc_gun", func(conf GunConfig) core.Gun {
		return NewGun(conf)
	}, func() GunConfig {
		return GunConfig{}
	})

	// Run Pandora's CLI context engine
	cli.Run()

	// 2. Automatically compute and display metrics once the test finishes
	fmt.Println("\n==============================================")
	fmt.Println("📊 GENERATING LATENCY PERCENTILES SUMMARY")
	fmt.Println("==============================================")
	load.PrintPercentiles("./phout.log")
}
