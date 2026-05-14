package load

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func PrintPercentiles(logPath string) {
	file, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("❌ Failed to parse log file: %v\n", err)
		return
	}
	defer file.Close()

	var latencies []float64
	var discardedCount int
	var rateLimitedCount int
	var timeoutCount int
	var successCount int

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 12 {
			continue
		}

		tag := fields[1]
		statusCode := fields[10] // Column 10 (Errno field) -> Overwritten as "999" by SetErr()
		errCode := fields[11]    // Column 11 (ProtoCode field) -> Holds your custom "888" marker cleanly

		// 🔍 THE CORRECTION: Evaluate errCode (Column 11) for your custom marker
		if errCode == "888" {
			rateLimitedCount++
			continue // Skip adding rate-limited requests to processing latency arrays
		}

		// Track framework load dropping behaviors
		if tag == "discarded" || statusCode == "777" || errCode == "777" {
			discardedCount++
			continue
		}

		// Separate true timeouts from success profiles
		if statusCode == "999" || errCode == "999" {
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

	totalRequests := len(latencies) + discardedCount + rateLimitedCount
	if len(latencies) == 0 {
		fmt.Println("⚠️ No active network transactions recorded in log file.")
		return
	}

	sort.Float64s(latencies)

	p50Idx := int(float64(len(latencies)) * 0.50)
	p95Idx := int(float64(len(latencies)) * 0.95)
	p99Idx := int(float64(len(latencies)) * 0.99)

	if p95Idx >= len(latencies) {
		p95Idx = len(latencies) - 1
	}
	if p99Idx >= len(latencies) {
		p99Idx = len(latencies) - 1
	}

	fmt.Printf("Total Requests Generated : %d\n", totalRequests)
	fmt.Printf("  └─ Successful Responses: %d\n", successCount)
	fmt.Printf("  └─ gRPC Timeouts (999) : %d\n", timeoutCount)
	fmt.Printf("  └─ Rate limited        : %d\n", rateLimitedCount)
	fmt.Printf("  └─ Pool Drops (777)    : %d\n\n", discardedCount)
	fmt.Println("Execution Latencies Distribution (On-Wire Traffic):")
	fmt.Printf("  ⚡ p50 (Median) : %.2f ms\n", latencies[p50Idx])
	fmt.Printf("  ⚠️ p95          : %.2f ms\n", latencies[p95Idx])
	fmt.Printf("  🚨 p99          : %.2f ms\n", latencies[p99Idx])
	fmt.Println("==============================================")
}
