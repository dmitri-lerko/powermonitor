package power

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Reading struct {
	Timestamp      time.Time `json:"timestamp"`
	PowerDraw      float64   `json:"power_draw"`
	CpuWatts       float64   `json:"cpu_watts"`
	GpuWatts       float64   `json:"gpu_watts"`
	AneWatts       float64   `json:"ane_watts"`
	IsCharging     bool      `json:"is_charging"`
	BatteryPct     float64   `json:"battery_pct"`
	Source         string    `json:"source"`
	AdapterW       float64   `json:"adapter_w"`
	ChargingWatts  float64   `json:"charging_watts"`
	SystemWatts    float64   `json:"system_watts"`
}

type PowerResult struct {
	TotalW   float64
	CpuW     float64
	GpuW     float64
	AneW     float64
}

func GetPowerDraw() (float64, error) {
	// Try powermetrics directly first
	cmd := exec.Command("powermetrics", "--samplers", "cpu_power", "-n", "1", "-i", "1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		r := parsePowerOutput(output)
		return r.TotalW, nil
	}

	// If not running as root, try with sudo
	if os.Geteuid() != 0 {
		cmd = exec.Command("sudo", "-n", "powermetrics", "--samplers", "cpu_power", "-n", "1", "-i", "1")
		output, err = cmd.CombinedOutput()
		if err == nil {
			r := parsePowerOutput(output)
			return r.TotalW, nil
		}
	}

	return 0, fmt.Errorf("powermetrics failed (exit %v): %s", err, strings.TrimSpace(string(output)))
}

func GetPowerBreakdown() PowerResult {
	cmd := exec.Command("powermetrics", "--samplers", "cpu_power", "-n", "1", "-i", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if os.Geteuid() != 0 {
			cmd = exec.Command("sudo", "-n", "powermetrics", "--samplers", "cpu_power", "-n", "1", "-i", "1")
			output, err = cmd.CombinedOutput()
			if err != nil {
				return PowerResult{}
			}
		} else {
			return PowerResult{}
		}
	}
	return parsePowerOutput(output)
}

func GetBatteryCapacity() float64 {
	cmd := exec.Command("pmset", "-g", "batt")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`(\d+) mAh`)
	m := re.FindStringSubmatch(string(output))
	if m != nil {
		mah, _ := strconv.ParseFloat(m[1], 64)
		// Apple batteries are typically ~11.4V nominal
		return mah * 11.4 / 1000.0 / 1000.0
	}
	return 0
}

var lastBatteryPct = -1.0
var lastBatteryTime = time.Time{}
var batteryCapacity = 0.0

func GetAdapterWattage() float64 {
	// Use pmset to check if AC power is connected
	cmd := exec.Command("pmset", "-g", "ps")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	outputStr := strings.ToLower(string(output))
	// Check if AC power is connected (adapter present)
	if strings.Contains(outputStr, "ac connected") || strings.Contains(outputStr, "ac power") {
		// Adapter is connected — try to get its rated wattage
		profilerCmd := exec.Command("system_profiler", "SPPowerDataType")
		profilerOutput, err := profilerCmd.Output()
		if err == nil {
			re := regexp.MustCompile(`Wattage \(W\):\s*([\d.]+)`)
			m := re.FindStringSubmatch(string(profilerOutput))
			if m != nil {
				w, _ := strconv.ParseFloat(m[1], 64)
				return w
			}
		}
		return -1 // AC connected but couldn't get wattage
	}
	return 0 // on battery, no adapter
}

func parsePowerOutput(output []byte) PowerResult {
	var r PowerResult
	lines := strings.Split(string(output), "\n")

	// Parse CPU Power
	for _, line := range lines {
		line = strings.TrimSpace(line)
		re := regexp.MustCompile(`CPU Power:\s*([\d.]+)\s*mW`)
		if m := re.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			r.CpuW = v / 1000.0
		}
		re = regexp.MustCompile(`GPU Power:\s*([\d.]+)\s*mW`)
		if m := re.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			r.GpuW = v / 1000.0
		}
		re = regexp.MustCompile(`ANE Power:\s*([\d.]+)\s*mW`)
		if m := re.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			r.AneW = v / 1000.0
		}
		re = regexp.MustCompile(`Combined Power.*?:\s*([\d.]+)\s*mW`)
		if m := re.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			r.TotalW = v / 1000.0
		}
	}
	return r
}

func GetBatteryStatus() (bool, float64, string, error) {
	cmd := exec.Command("pmset", "-g", "batt")
	output, err := cmd.Output()
	if err != nil {
		return false, 0, "", fmt.Errorf("pmset batt failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var charging bool
	var pct float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "charging") {
			charging = true
		}
		pctStr := extractPct(line)
		if pctStr != "" {
			pct, _ = strconv.ParseFloat(pctStr, 64)
		}
	}

	source := "Unknown"
	if strings.Contains(string(output), "AC Power") || strings.Contains(string(output), "ac connected") {
		source = "AC"
	} else if strings.Contains(string(output), "Battery Power") || strings.Contains(string(output), "discharging") {
		source = "Battery"
	}

	if !charging && source == "AC" {
		source = "AC (idle)"
	}

	return charging, pct, source, nil
}

func Collect() (Reading, error) {
	var r Reading
	r.Timestamp = time.Now()

	power := GetPowerBreakdown()
	r.PowerDraw = power.TotalW
	r.CpuWatts = power.CpuW
	r.GpuWatts = power.GpuW
	r.AneWatts = power.AneW
	r.AdapterW = GetAdapterWattage()

	charging, pct, source, err := GetBatteryStatus()
	if err != nil {
		r.Source = "battery status unavailable"
		r.BatteryPct = 0
		r.IsCharging = false
		return r, nil
	}
	r.IsCharging = charging
	r.BatteryPct = pct
	r.Source = source

	// Calculate charging power from battery % change
	if batteryCapacity == 0 {
		batteryCapacity = GetBatteryCapacity()
	}
	if charging && batteryCapacity > 0 && lastBatteryPct >= 0 {
		dt := r.Timestamp.Sub(lastBatteryTime).Seconds()
		if dt > 0 {
			pctChange := pct - lastBatteryPct
			if pctChange > 0 {
				// power (W) = energy (Wh) / time (h) = (pctChange/100 * capacity_Wh) / (dt/3600)
				r.ChargingWatts = (pctChange / 100.0 * batteryCapacity) / (dt / 3600.0)
				r.SystemWatts = power.TotalW - r.ChargingWatts
				if r.SystemWatts < 0 {
					r.SystemWatts = power.TotalW
					r.ChargingWatts = 0
				}
			}
		}
	}

	// Update tracking
	lastBatteryPct = pct
	lastBatteryTime = r.Timestamp

	return r, nil
}

func extractPowerLine(output []byte) string {
	lines := strings.Split(string(output), "\n")
	// Try new format: "Combined Power (CPU + GPU + ANE): 5731 mW"
	re := regexp.MustCompile(`Combined Power.*?:\s*([\d.]+)\s*mW`)
	for i := len(lines) - 1; i >= 0; i-- {
		m := re.FindStringSubmatch(lines[i])
		if m != nil {
			return m[0]
		}
	}
	// Try old format: "System Power (( X.X )) W"
	re = regexp.MustCompile(`System Power \(\(\s*[\d.]+\s*\)\s*W`)
	for i := len(lines) - 1; i >= 0; i-- {
		m := re.FindStringSubmatch(lines[i])
		if m != nil {
			return m[0]
		}
	}
	// Try CPU Power as fallback
	re = regexp.MustCompile(`CPU Power:\s*([\d.]+)\s*mW`)
	for i := len(lines) - 1; i >= 0; i-- {
		m := re.FindStringSubmatch(lines[i])
		if m != nil {
			return m[0]
		}
	}
	return ""
}

func parseWatts(line string) (float64, error) {
	// New format: "Combined Power (CPU + GPU + ANE): 5731 mW" or "CPU Power: 3867 mW"
	re := regexp.MustCompile(`([\d.]+)\s*mW`)
	m := re.FindStringSubmatch(line)
	if m != nil {
		mw, _ := strconv.ParseFloat(m[1], 64)
		return mw / 1000.0, nil
	}
	// Old format: "System Power (( X.X )) W"
	re = regexp.MustCompile(`\(\s*([\d.]+)\s*\)\s*W`)
	m = re.FindStringSubmatch(line)
	if m != nil {
		return strconv.ParseFloat(m[1], 64)
	}
	return 0, fmt.Errorf("no watts in: %s", line)
}

func extractPct(line string) string {
	re := regexp.MustCompile(`(\d+)%`)
	m := re.FindStringSubmatch(line)
	if m != nil {
		return m[1]
	}
	return ""
}
