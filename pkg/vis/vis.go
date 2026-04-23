package vis

import (
	"fmt"
	"math"
	"strings"

	"github.com/dmitri-lerko/powermon/pkg/db"
	"github.com/dmitri-lerko/powermon/pkg/power"
)

const (
	sparklineWidth = 60
	barWidth       = 3
)

func LiveView(readings []power.Reading) string {
	if len(readings) == 0 {
		return "No readings yet..."
	}

	last := readings[len(readings)-1]

	var statusIcon string
	if last.IsCharging {
		statusIcon = "⚡"
	} else {
		statusIcon = "🔋"
	}

	source := last.Source
	if source == "" {
		source = "?"
	}

	// Build last 60 readings sparkline
	var sparkData []float64
	start := len(readings) - 60
	if start < 0 {
		start = 0
	}
	for _, r := range readings[start:] {
		if r.PowerDraw >= 0 {
			sparkData = append(sparkData, r.PowerDraw)
		}
	}
	spark := sparklineFromData(sparkData, sparklineWidth)

	// Recent readings table
	var lines []string
	if last.PowerDraw < 0 {
		lines = append(lines, "  Power: -- W (powermetrics needs elevated privileges)")
		lines = append(lines, "  Run: sudo powermon live")
	} else if last.PowerDraw > 0 {
		if last.ChargingWatts > 0.1 {
			lines = append(lines, fmt.Sprintf("  %s  Total: %.1f W", statusIcon, last.PowerDraw))
			lines = append(lines, fmt.Sprintf("         System: %.1f W  Battery: %.1f W", last.SystemWatts, last.ChargingWatts))
			lines = append(lines, fmt.Sprintf("         CPU: %.1f W  GPU: %.1f W  Neural Accelerator (ANE): %.1f W", last.CpuWatts, last.GpuWatts, last.AneWatts))
		} else if last.IsCharging {
			lines = append(lines, fmt.Sprintf("  %s  Total: %.1f W", statusIcon, last.PowerDraw))
			lines = append(lines, "         Charging (split updates when battery % changes)")
			lines = append(lines, fmt.Sprintf("         CPU: %.1f W  GPU: %.1f W  Neural Accelerator (ANE): %.1f W", last.CpuWatts, last.GpuWatts, last.AneWatts))
		} else {
			lines = append(lines, fmt.Sprintf("  %s  Total: %.1f W", statusIcon, last.PowerDraw))
			lines = append(lines, fmt.Sprintf("         CPU: %.1f W  GPU: %.1f W  Neural Accelerator (ANE): %.1f W", last.CpuWatts, last.GpuWatts, last.AneWatts))
			lines = append(lines, "         Battery: idle")
		}
		if last.AdapterW > 0 {
			lines = append(lines, fmt.Sprintf("         Adapter: %.0f W", last.AdapterW))
		}
	} else {
		lines = append(lines, fmt.Sprintf("  %s  Power: %.1f W", statusIcon, last.PowerDraw))
	}
	lines = append(lines, fmt.Sprintf("  Battery: %.0f%%  Source: %s", last.BatteryPct, source))
	lines = append(lines, fmt.Sprintf("  Time: %s", last.Timestamp.Format("15:04:05")))
	lines = append(lines, "")
	lines = append(lines, "  Power (W)")
	lines = append(lines, "  "+spark)
	lines = append(lines, "  "+strings.Repeat("-", sparklineWidth))

	if len(readings) > 1 {
		minW := math.Inf(1)
		maxW := math.Inf(-1)
		sumW := 0.0
		count := 0
		for _, r := range readings[start:] {
			if r.PowerDraw < 0 {
				continue
			}
			if r.PowerDraw < minW {
				minW = r.PowerDraw
			}
			if r.PowerDraw > maxW {
				maxW = r.PowerDraw
			}
			sumW += r.PowerDraw
			count++
		}
		if count > 0 {
			avgW := sumW / float64(count)
			lines = append(lines, fmt.Sprintf("  Min: %.1f  Avg: %.1f  Max: %.1f W", minW, avgW, maxW))
		}
	}

	lines = append(lines, fmt.Sprintf("  %d samples", len(readings)))
	return strings.Join(lines, "\n")
}

func HistoricalViewDay(dbStore *db.Store, days int) string {
	agg, err := dbStore.AggregatedByHour(days)
	if err != nil || len(agg) == 0 {
		return "No historical data. Run 'live' mode first to collect data."
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("  Hourly Power Usage (last %d days)", days))
	lines = append(lines, "")

	// Find max for scaling
	var maxW float64
	for _, a := range agg {
		if a.AvgWatts > maxW {
			maxW = a.AvgWatts
		}
	}
	if maxW == 0 {
		maxW = 1
	}

	// Group by day
	currentDay := ""
	for _, a := range agg {
		if a.Day != currentDay {
			if currentDay != "" {
				lines = append(lines, "")
			}
			lines = append(lines, fmt.Sprintf("  %s", a.Day))
			currentDay = a.Day
		}
		bar := barChart(float64(a.AvgWatts), maxW, 40)
		lines = append(lines, fmt.Sprintf("    %s  %.1f W (avg) / %.1f W (max)  %.0f Wh", bar, a.AvgWatts, a.MaxWatts, a.TotalWh))
	}

	return strings.Join(lines, "\n")
}

func HistoricalViewWeek(dbStore *db.Store, weeks int) string {
	agg, err := dbStore.AggregatedByDay(weeks * 7)
	if err != nil || len(agg) == 0 {
		return "No historical data. Run 'live' mode first to collect data."
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("  Daily Power Usage (last %d weeks)", weeks))
	lines = append(lines, "")

	var maxW float64
	for _, a := range agg {
		if a.TotalWh > maxW {
			maxW = a.TotalWh
		}
	}
	if maxW == 0 {
		maxW = 1
	}

	for _, a := range agg {
		bar := barChart(a.TotalWh, maxW, 40)
		lines = append(lines, fmt.Sprintf("  %s  %s  %.1f Wh", bar, a.Day, a.TotalWh))
	}

	return strings.Join(lines, "\n")
}

func SparklineView(readings []power.Reading) string {
	if len(readings) < 2 {
		return "Need more data for sparkline."
	}

	var data []float64
	start := len(readings) - 120
	if start < 0 {
		start = 0
	}
	for _, r := range readings[start:] {
		if r.PowerDraw >= 0 {
			data = append(data, r.PowerDraw)
		}
	}

	spark := sparklineFromData(data, 120)

	var lines []string
	lines = append(lines, "  Power Draw Sparkline (last "+fmt.Sprintf("%d", len(data))+"s)")
	lines = append(lines, "  "+spark)

	if len(data) > 0 {
		var minW, maxW, sumW float64
		minW = math.Inf(1)
		maxW = math.Inf(-1)
		for _, v := range data {
			if v < minW {
				minW = v
			}
			if v > maxW {
				maxW = v
			}
			sumW += v
		}
		avgW := sumW / float64(len(data))
		lines = append(lines, fmt.Sprintf("  Min: %.1f W  Avg: %.1f W  Max: %.1f W", minW, avgW, maxW))
	} else {
		lines = append(lines, "  (no wattage data - powermetrics requires sudo)")
	}

	return strings.Join(lines, "\n")
}

func sparklineFromData(data []float64, width int) string {
	if len(data) == 0 {
		return strings.Repeat(" ", width)
	}

	var minV, maxV float64
	minV = math.Inf(1)
	maxV = math.Inf(-1)
	for _, v := range data {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if maxV == minV {
		maxV = minV + 1
	}

	ranges := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	step := float64(len(ranges)) / (maxV - minV)

	var chars []string
	for _, v := range data {
		idx := int(math.Round((v - minV) * step))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(ranges) {
			idx = len(ranges) - 1
		}
		chars = append(chars, ranges[idx])
	}

	// Trim to width
	if len(chars) > width {
		chars = chars[len(chars)-width:]
	}
	return strings.Join(chars, "")
}

func barChart(value, maxVal float64, width int) string {
	if maxVal == 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(width) * value / maxVal)
	if filled < 1 && value > 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return "█" + strings.Repeat("█", filled-1) + strings.Repeat("░", width-filled)
}
