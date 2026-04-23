package power

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TriggerANELoad runs synthetic ANE workload by invoking a helper binary.
// Returns the number of iterations completed, or -1 on error.
func TriggerANELoad(iterations int) int {
	// Try to find the ANE load helper binary
	paths := []string{
		"/opt/homebrew/bin/powermon_ane_load",
		"/usr/local/bin/powermon_ane_load",
	}
	
	// Also try relative to current working directory
	if cwd, err := os.Getwd(); err == nil {
		paths = append([]string{cwd + "/tools/ane_load"}, paths...)
	}
	
	for _, path := range paths {
		// Check if file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		
		cmd := exec.Command(path, fmt.Sprintf("%d", iterations))
		output, err := cmd.CombinedOutput()
		if err == nil {
			// Parse the result
			result := strings.TrimSpace(string(output))
			var completed int
			fmt.Sscanf(result, "%d", &completed)
			return completed
		}
	}
	
	return -1
}

// TriggerANELoadWithTimeout runs synthetic ANE workload with a timeout.
// Returns the number of iterations completed, or -1 on error/timeout.
func TriggerANELoadWithTimeout(iterations int, timeout time.Duration) int {
	// Try to find the ANE load helper binary
	paths := []string{
		"/opt/homebrew/bin/powermon_ane_load",
		"/usr/local/bin/powermon_ane_load",
	}
	
	// Also try relative to current working directory
	if cwd, err := os.Getwd(); err == nil {
		paths = append([]string{cwd + "/tools/ane_load"}, paths...)
	}
	
	for _, path := range paths {
		// Check if file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		
		// Use a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, path, fmt.Sprintf("%d", iterations))
		output, err := cmd.CombinedOutput()
		if err == nil {
			// Parse the result
			result := strings.TrimSpace(string(output))
			var completed int
			fmt.Sscanf(result, "%d", &completed)
			return completed
		}
	}
	
	return -1
}

// GetANEPower reads current ANE power from powermetrics.
// Returns ANE power in watts, or 0 if unavailable.
func GetANEPower(useSudo bool) (float64, error) {
	var output []byte
	var err error
	
	// Try with sudo first (powermetrics requires root)
	if useSudo {
		cmd := exec.Command("sudo", "powermetrics", "--samplers", "cpu_power", "-n", "1", "-i", "1")
		output, err = cmd.CombinedOutput()
		if err == nil {
			return parseANEPower(output)
		}
	}
	
	// Try without sudo
	cmd := exec.Command("powermetrics", "--samplers", "cpu_power", "-n", "1", "-i", "1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	
	return parseANEPower(output)
}

func parseANEPower(output []byte) (float64, error) {
	// Parse ANE Power from output
	// Format: "ANE Power: XXX mW"
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ANE Power:") {
			continue
		}
		// Extract the value after "ANE Power: "
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		valStr := strings.TrimSpace(parts[1])
		// Extract just the number (handle "XXX mW" format)
		var val float64
		n, err := fmt.Sscanf(valStr, "%f", &val)
		if err == nil && n == 1 {
			return val / 1000.0, nil // Convert mW to W
		}
	}
	
	return 0, nil
}

// BenchmarkConfig holds configuration for the ANE benchmark.
type BenchmarkConfig struct {
	Duration   time.Duration
	Iterations int
	Interval   time.Duration
}

// DefaultBenchmarkConfig returns default configuration for ANE benchmark.
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		Duration:   10 * time.Second,
		Iterations: 200,
		Interval:   1 * time.Second,
	}
}
