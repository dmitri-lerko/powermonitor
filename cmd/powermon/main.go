package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/dmitri-lerko/powermon/pkg/db"
	"github.com/dmitri-lerko/powermon/pkg/power"
	"github.com/dmitri-lerko/powermon/pkg/vis"
)

var defaultDB = "data/powermon.db"

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		defaultDB = filepath.Join(home, ".powermon", "data", "powermon.db")
	}
}

func main() {
	runtime.GOMAXPROCS(1)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "live":
		cmdLive(os.Args[2:])
	case "view":
		cmdView(os.Args[2:])
	case "sparkline":
		cmdSparkline(os.Args[2:])
	case "hourly":
		cmdHourly(os.Args[2:])
	case "daily":
		cmdDaily(os.Args[2:])
	case "today":
		cmdToday(os.Args[2:])
	case "status":
		cmdStatus()
	case "dump":
		cmdDump(os.Args[2:])
	case "bench":
		cmdBench(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdLive(args []string) {
	interval := 1 * time.Second
	dbPath := defaultDB

	fs := flag.NewFlagSet("live", flag.ExitOnError)
	fs.DurationVar(&interval, "interval", interval, "sample interval")
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Power Monitor - Live View")
	fmt.Printf("  DB: %s\n", dbPath)
	fmt.Printf("  Interval: %s\n", interval)
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var readings []power.Reading
	count := 0

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nCollected", count, "readings.")
			return
		case <-ticker.C:
			r, err := power.Collect()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error collecting: %v\n", err)
				continue
			}

			if err := store.Save(r); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving: %v\n", err)
			}

			readings = append(readings, r)
			count++

			fmt.Print("\033[H\033[2J")
			fmt.Println(vis.LiveView(readings))
		}
	}
}

func cmdView(args []string) {
	dbPath := defaultDB

	fs := flag.NewFlagSet("view", flag.ExitOnError)
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	readings, err := store.Recent(120)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(vis.LiveView(readings))
}

func cmdSparkline(args []string) {
	dbPath := defaultDB

	fs := flag.NewFlagSet("sparkline", flag.ExitOnError)
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	readings, err := store.Recent(300)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(vis.SparklineView(readings))
}

func cmdHourly(args []string) {
	dbPath := defaultDB
	days := 7

	fs := flag.NewFlagSet("hourly", flag.ExitOnError)
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.IntVar(&days, "days", days, "number of days")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Println(vis.HistoricalViewDay(store, days))
}

func cmdDaily(args []string) {
	dbPath := defaultDB
	weeks := 4

	fs := flag.NewFlagSet("daily", flag.ExitOnError)
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.IntVar(&weeks, "weeks", weeks, "number of weeks")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Println(vis.HistoricalViewWeek(store, weeks))
}

func cmdToday(args []string) {
	dbPath := defaultDB

	fs := flag.NewFlagSet("today", flag.ExitOnError)
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	stats, err := store.TodayStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Today's Power Stats")
	fmt.Println("===================")
	fmt.Printf("  Avg Power: %.1f W\n", stats.AvgWatts)
	fmt.Printf("  Max Power: %.1f W\n", stats.MaxWatts)
	fmt.Printf("  Min Power: %.1f W\n", stats.MinWatts)
	fmt.Printf("  Total Energy: %.1f Wh\n", stats.TotalWh)
	fmt.Printf("  Samples: %d\n", stats.Samples)
}

func cmdStatus() {
	r, err := power.Collect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check powermetrics availability
	pmCmd := exec.Command("powermetrics", "--samplers", "cpu_power", "-n", "1")
	pmOut, pmErr := pmCmd.CombinedOutput()
	fmt.Printf("powermetrics: %s\n", statusStr(pmErr == nil))
	if pmErr != nil {
		// Check if it's a permission issue
		if strings.Contains(string(pmOut), "superuser") || strings.Contains(string(pmOut), "root") {
			fmt.Printf("  -> Requires root privileges\n")
			fmt.Printf("  -> Run: sudo powermon live\n")
		} else {
			fmt.Printf("  -> Error: %s\n", strings.TrimSpace(string(pmOut)))
		}
	}

	var charging string
	if r.IsCharging {
		charging = "⚡ Charging"
	} else {
		charging = "🔋 Discharging"
	}

	fmt.Printf("Power: %.1f W\n", r.PowerDraw)
	if r.PowerDraw > 0 {
		if r.ChargingWatts > 0.1 {
			fmt.Printf("  System: %.1f W | Battery: %.1f W\n", r.SystemWatts, r.ChargingWatts)
		}
		fmt.Printf("  CPU: %.1f W  GPU: %.1f W  Neural Accelerator (ANE): %.1f W\n", r.CpuWatts, r.GpuWatts, r.AneWatts)
	}
	fmt.Printf("Battery: %.0f%%\n", r.BatteryPct)
	fmt.Printf("Source: %s\n", r.Source)
	fmt.Printf("Status: %s\n", charging)
	if r.AdapterW > 0 {
		fmt.Printf("Adapter: %.0f W\n", r.AdapterW)
	}
	fmt.Printf("UID: %d (effective: %d)\n", os.Getuid(), os.Geteuid())
}

func statusStr(ok bool) string {
	if ok {
		return "OK"
	}
	return "FAIL"
}

func cmdBench(args []string) {
	duration := 10 * time.Second
	iterations := 200
	interval := 1 * time.Second
	dbPath := defaultDB
	useSudo := false

	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	fs.DurationVar(&duration, "duration", duration, "benchmark duration")
	fs.IntVar(&iterations, "iterations", iterations, "number of ANE workload iterations")
	fs.DurationVar(&interval, "interval", interval, "reporting interval")
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.BoolVar(&useSudo, "sudo", false, "use sudo for powermetrics (requires password)")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Println("=== ANE Benchmark ===")
	fmt.Printf("Duration: %s | Iterations: %d | Interval: %s\n\n", duration, iterations, interval)

	// Take baseline reading
	fmt.Println("Taking baseline ANE power...")
	time.Sleep(2 * time.Second)
	baseline, err := power.GetANEPower(useSudo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: couldn't read ANE power: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run with -sudo flag for accurate readings (requires password)\n\n")
		baseline = 0
	}
	fmt.Printf("Baseline ANE: %.1f W\n\n", baseline)

	// Start benchmark
	fmt.Println("Triggering ANE load...")
	fmt.Println(strings.Repeat("-", 60))

	startTime := time.Now()
	peakANE := 0.0
	totalANE := 0.0
	count := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		time.Sleep(duration)
		close(done)
	}()

	for {
		select {
		case <-done:
			goto done
		case <-ticker.C:
			// Trigger ANE load in chunks
			go power.TriggerANELoadWithTimeout(iterations/int(duration.Seconds()/interval.Seconds())+1, 5*time.Second)

			// Read ANE power
			anePower, err := power.GetANEPower(useSudo)
			if err == nil && anePower > 0 {
				if anePower > peakANE {
					peakANE = anePower
				}
				totalANE += anePower
				count++
				fmt.Printf("  ANE: %7.1f W  (peak: %7.1f W)\n", anePower, peakANE)
			} else {
				fmt.Println("  (couldn't read ANE power - try with -sudo)")
			}
		}
	}

done:
	fmt.Println(strings.Repeat("-", 60))

	// Post-baseline reading
	fmt.Println("\nTaking post-baseline ANE power...")
	time.Sleep(2 * time.Second)
	postBaseline, err := power.GetANEPower(useSudo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: couldn't read post-baseline: %v\n", err)
		postBaseline = 0
	}

	avgANE := 0.0
	if count > 0 {
		avgANE = totalANE / float64(count)
	}

	fmt.Println("\n=== Results ===")
	fmt.Printf("  Baseline ANE:  %.1f W\n", baseline)
	fmt.Printf("  Peak ANE:      %.1f W\n", peakANE)
	fmt.Printf("  Average ANE:   %.1f W\n", avgANE)
	fmt.Printf("  Post-baseline: %.1f W\n", postBaseline)
	fmt.Printf("  Samples:       %d\n", count)
	fmt.Printf("  Duration:      %s\n", time.Since(startTime).Round(time.Millisecond))
}

func cmdDump(args []string) {
	dbPath := defaultDB
	limit := 50

	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	fs.StringVar(&dbPath, "db", defaultDB, "database path")
	fs.IntVar(&limit, "n", limit, "number of rows")
	fs.Parse(args)

	store, err := db.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	readings, err := store.Recent(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%-22s %7s %7s %7s %9s %6s %4s %8s %8s %s\n", "TIME", "TOTAL", "CPU", "GPU", "NEURAL", "BAT%", "CHG", "SYS", "CHG W", "SOURCE")
	fmt.Println(strings.Repeat("-", 110))
	for _, r := range readings {
		chg := " "
		if r.IsCharging {
			chg = "⚡"
		}
		sysW := r.SystemWatts
		chrW := r.ChargingWatts
		if sysW < 0 {
			sysW = r.PowerDraw
			chrW = 0
		}
		fmt.Printf("%-22s %7.1f %7.1f %7.1f %8.1f W %5.0f%% %3s %7.1f W %7.1f W %s\n",
			r.Timestamp.Format("2006-01-02 15:04:05"),
			r.PowerDraw,
			r.CpuWatts,
			r.GpuWatts,
			r.AneWatts,
			r.BatteryPct,
			chg,
			sysW,
			chrW,
			r.Source,
		)
	}
}

func printUsage() {
	fmt.Println(`powermon - macOS power monitor

Usage:
  powermon <command> [arguments]

Commands:
  live      Live monitoring with real-time visualization (Ctrl+C to stop)
  view      Show recent readings visualization
  sparkline Show sparkline view of recent power draw
  hourly    Show hourly aggregated power usage
  daily     Show daily aggregated power usage
  today     Show today's power statistics
  status    Show current power status
  bench     Run ANE benchmark with synthetic load
  dump      Dump raw readings to terminal

Options:
  -db string   Database path (default "data/powermon.db")

Examples:
  powermon live -interval 2s          # Sample every 2 seconds
  powermon live -db /tmp/power.db     # Use custom database
  powermon sparkline                  # View sparkline
  powermon hourly -days 14            # View 14 days of hourly data
  powermon daily -weeks 8             # View 8 weeks of daily data
  powermon today                      # Today's stats
  powermon status                     # Current status
  powermon bench                      # Run ANE benchmark (requires sudo)
  powermon bench -duration 15s        # Benchmark for 15 seconds
  sudo powermon bench                 # Run with sudo for ANE readings
`)
}
