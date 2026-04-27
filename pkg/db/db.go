package db

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyber-nic/powermon/pkg/power"
)

type Store struct {
	path string
}

func NewStore(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	// Initialize database schema
	queries := []string{
		`CREATE TABLE IF NOT EXISTS readings (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp DATETIME NOT NULL, power_draw REAL NOT NULL, cpu_watts REAL NOT NULL, gpu_watts REAL NOT NULL, ank_watts REAL NOT NULL, charging_watts REAL NOT NULL, system_watts REAL NOT NULL, is_charging INTEGER NOT NULL, battery_pct REAL NOT NULL, source TEXT NOT NULL, adapter_w REAL NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_readings_ts ON readings(timestamp);`,
	}
	for _, q := range queries {
		if err := execSQLite(path, q); err != nil {
			return nil, fmt.Errorf("init schema: %w", err)
		}
	}

	return &Store{path: path}, nil
}

func execSQLite(dbPath, query string) error {
	cmd := exec.Command("sqlite3", dbPath, query)
	_, err := cmd.CombinedOutput()
	return err
}

func execSQLiteOutput(dbPath, query string) (string, error) {
	cmd := exec.Command("sqlite3", "-separator", "|", dbPath, query)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (s *Store) Save(r power.Reading) error {
	query := fmt.Sprintf(
		"INSERT INTO readings (timestamp, power_draw, cpu_watts, gpu_watts, ank_watts, charging_watts, system_watts, is_charging, battery_pct, source, adapter_w) VALUES ('%s', %.6f, %.6f, %.6f, %.6f, %.6f, %.6f, %d, %.2f, '%s', %.2f);",
		r.Timestamp.Format("2006-01-02 15:04:05"),
		r.PowerDraw,
		r.CpuWatts,
		r.GpuWatts,
		r.AneWatts,
		r.ChargingWatts,
		r.SystemWatts,
		btoi(r.IsCharging),
		r.BatteryPct,
		strings.ReplaceAll(r.Source, "'", "''"),
		r.AdapterW,
	)
	return execSQLite(s.path, query)
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) Recent(n int) ([]power.Reading, error) {
	query := fmt.Sprintf(
		"SELECT timestamp, power_draw, cpu_watts, gpu_watts, ank_watts, charging_watts, system_watts, is_charging, battery_pct, source, adapter_w FROM readings ORDER BY timestamp DESC LIMIT %d;",
		n,
	)
	output, err := execSQLiteOutput(s.path, query)
	if err != nil {
		return nil, err
	}

	var readings []power.Reading
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 11 {
			continue
		}
		var r power.Reading
		var ts string
		var charging int
		ts = parts[0]
		r.PowerDraw, _ = strconv.ParseFloat(parts[1], 64)
		r.CpuWatts, _ = strconv.ParseFloat(parts[2], 64)
		r.GpuWatts, _ = strconv.ParseFloat(parts[3], 64)
		r.AneWatts, _ = strconv.ParseFloat(parts[4], 64)
		r.ChargingWatts, _ = strconv.ParseFloat(parts[5], 64)
		r.SystemWatts, _ = strconv.ParseFloat(parts[6], 64)
		charging, _ = strconv.Atoi(parts[7])
		r.BatteryPct, _ = strconv.ParseFloat(parts[8], 64)
		r.Source = parts[9]
		r.AdapterW, _ = strconv.ParseFloat(parts[10], 64)
		r.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		r.IsCharging = charging == 1
		readings = append(readings, r)
	}
	// reverse to chronological
	for i, j := 0, len(readings)-1; i < j; i, j = i+1, j-1 {
		readings[i], readings[j] = readings[j], readings[i]
	}
	return readings, nil
}

func (s *Store) AggregatedByHour(days int) ([]HourAgg, error) {
	start := time.Now().AddDate(0, 0, -days)
	query := fmt.Sprintf(`
		SELECT date(timestamp), strftime('%%H', timestamp), AVG(power_draw), MAX(power_draw), SUM(power_draw)
		FROM readings
		WHERE timestamp >= '%s'
		GROUP BY date(timestamp), strftime('%%H', timestamp)
		ORDER BY date(timestamp), strftime('%%H', timestamp);
	`, start.Format("2006-01-02 15:04:05"))

	output, err := execSQLiteOutput(s.path, query)
	if err != nil {
		return nil, err
	}

	var result []HourAgg
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}
		var h HourAgg
		h.Day = parts[0]
		h.Hour = parts[1]
		h.AvgWatts, _ = strconv.ParseFloat(parts[2], 64)
		h.MaxWatts, _ = strconv.ParseFloat(parts[3], 64)
		h.TotalWh, _ = strconv.ParseFloat(parts[4], 64)
		result = append(result, h)
	}
	return result, nil
}

func (s *Store) AggregatedByDay(days int) ([]DayAgg, error) {
	start := time.Now().AddDate(0, 0, -days)
	query := fmt.Sprintf(`
		SELECT date(timestamp), AVG(power_draw), MAX(power_draw), SUM(power_draw)
		FROM readings
		WHERE timestamp >= '%s'
		GROUP BY date(timestamp)
		ORDER BY date(timestamp);
	`, start.Format("2006-01-02 15:04:05"))

	output, err := execSQLiteOutput(s.path, query)
	if err != nil {
		return nil, err
	}

	var result []DayAgg
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		var d DayAgg
		d.Day = parts[0]
		d.AvgWatts, _ = strconv.ParseFloat(parts[1], 64)
		d.MaxWatts, _ = strconv.ParseFloat(parts[2], 64)
		d.TotalWh, _ = strconv.ParseFloat(parts[3], 64)
		result = append(result, d)
	}
	return result, nil
}

func (s *Store) TodayStats() (TodayStats, error) {
	today := time.Now().Format("2006-01-02")
	query := fmt.Sprintf(`
		SELECT AVG(power_draw), MAX(power_draw), MIN(power_draw), SUM(power_draw), COUNT(*)
		FROM readings
		WHERE date(timestamp) = '%s';
	`, today)

	output, err := execSQLiteOutput(s.path, query)
	if err != nil {
		return TodayStats{}, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return TodayStats{}, nil
	}

	parts := strings.Split(output, "|")
	if len(parts) < 5 {
		return TodayStats{}, fmt.Errorf("unexpected output")
	}

	var stats TodayStats
	stats.AvgWatts, _ = strconv.ParseFloat(parts[0], 64)
	stats.MaxWatts, _ = strconv.ParseFloat(parts[1], 64)
	stats.MinWatts, _ = strconv.ParseFloat(parts[2], 64)
	stats.TotalWh, _ = strconv.ParseFloat(parts[3], 64)
	stats.Samples, _ = strconv.Atoi(parts[4])
	return stats, nil
}

type HourAgg struct {
	Day      string
	Hour     string
	AvgWatts float64
	MaxWatts float64
	TotalWh  float64
}

type DayAgg struct {
	Day      string
	AvgWatts float64
	MaxWatts float64
	TotalWh  float64
}

type TodayStats struct {
	AvgWatts float64
	MaxWatts float64
	MinWatts float64
	TotalWh  float64
	Samples  int
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
