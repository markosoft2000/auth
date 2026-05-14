package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"
	"github.com/spf13/afero"
	"github.com/yandex/pandora/cli"
	"github.com/yandex/pandora/core"
	"github.com/yandex/pandora/core/aggregator/netsample"
	coreimport "github.com/yandex/pandora/core/import"
	"github.com/yandex/pandora/core/register"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Ammo defines the structure of your test data
type Ammo struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	AppID    string `json:"app_id"`
}

// ID returns a unique identifier for the ammo
func (a *Ammo) ID() string {
	return a.Email
}

// GunConfig defines configurations passed via your load.yaml
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

// NewGun instantiates your custom gRPC load generator
func NewGun(conf GunConfig) *Gun {
	return &Gun{
		conf: conf,
		tag:  "login_test",
	}
}

// Bind configures dependencies and dials the target server
func (g *Gun) Bind(aggr core.Aggregator, deps core.GunDeps) error {
	g.aggr = aggr

	// Pull target directly from GunConfig resolved via your YAML file
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &authv1.LoginRequest{
		Email:    customAmmo.Email,
		Password: customAmmo.Password,
		AppId:    customAmmo.AppID,
		Ip:       "127.0.0.1",
	}

	// Track start time
	startTime := time.Now()
	_, err := g.client.Login(ctx, req)

	// Record performance metric parameters
	sample.SetLatency(time.Since(startTime))
	if err != nil {
		sample.SetErr(err)
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
	printPercentiles("./phout.log")
}

func printPercentiles(logPath string) {
	file, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("❌ Failed to parse log file: %v\n", err)
		return
	}
	defer file.Close()

	var latencies []float64
	var discardedCount int
	var timeoutCount int
	var successCount int

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 11 {
			continue
		}

		tag := fields[1]
		errCode := fields[10]

		// Track framework load dropping behaviors
		if tag == "discarded" || errCode == "777" {
			discardedCount++
			continue
		}

		// Track hard network deadlines (5-second timeouts)
		if errCode == "999" {
			timeoutCount++
		} else {
			successCount++
		}

		// Column 6 represents Net Roundtrip latency in microseconds
		netLatMicro, err := strconv.ParseFloat(fields[5], 64)
		if err == nil && netLatMicro > 0 {
			latencies = append(latencies, netLatMicro/1000.0) // Convert to milliseconds
		}
	}

	totalRequests := len(latencies) + discardedCount
	if len(latencies) == 0 {
		fmt.Println("⚠️ No active network transactions recorded in log file.")
		return
	}

	sort.Float64s(latencies)

	p50Idx := int(float64(len(latencies)) * 0.50)
	p95Idx := int(float64(len(latencies)) * 0.95)
	p99Idx := int(float64(len(latencies)) * 0.99)

	// Ensure boundary safety for low request counts
	if p95Idx >= len(latencies) {
		p95Idx = len(latencies) - 1
	}
	if p99Idx >= len(latencies) {
		p99Idx = len(latencies) - 1
	}

	fmt.Printf("Total Requests Generated : %d\n", totalRequests)
	fmt.Printf("  └─ Successful Responses: %d\n", successCount)
	fmt.Printf("  └─ gRPC Timeouts (999) : %d\n", timeoutCount)
	fmt.Printf("  └─ Pool Drops (777)    : %d\n\n", discardedCount)
	fmt.Println("Execution Latencies Distribution (On-Wire Traffic):")
	fmt.Printf("  ⚡ p50 (Median) : %.2f ms\n", latencies[p50Idx])
	fmt.Printf("  ⚠️ p95          : %.2f ms\n", latencies[p95Idx])
	fmt.Printf("  🚨 p99          : %.2f ms\n", latencies[p99Idx])
	fmt.Println("==============================================")
}
