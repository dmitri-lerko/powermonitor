# powermon

macOS power draw and battery monitoring CLI tool. Samples CPU, GPU, and Neural Accelerator / ANE power every second, stores data in SQLite, and visualizes live and historical usage in the terminal.

## Features

- **Live monitoring** with real-time sparkline visualization
- **Per-component breakdown**: CPU, GPU, and Neural Accelerator (Apple Neural Engine / ANE) power
- **Charging split**: Estimates power going to system vs battery charging
- **Historical views**: Hourly and daily aggregated charts
- **Battery tracking**: Charging state, percentage, and power source (AC/Battery)
- **Adapter info**: Detects connected charger wattage
- **SQLite storage**: All data persisted to `~/.powermon/data/powermon.db`
- **Zero dependencies**: Uses macOS CLI tools only (`powermetrics`, `pmset`, `system_profiler`, `sqlite3`)

## Installation

```bash
go install github.com/cyber-nic/powermon@latest
```

## Usage

### Power metrics (requires sudo)

`powermetrics` needs root privileges to read CPU power data. Set up passwordless sudo:

```bash
echo "$USER ALL=(root) NOPASSWD: /usr/bin/powermetrics" | sudo tee /etc/sudoers.d/powermon
```

### Commands

```bash
# Live monitoring (Ctrl+C to stop)
sudo powermon live
sudo powermon live -interval 2s    # Sample every 2 seconds

# Snapshot views
powermon status                     # Current power/battery status
powermon view                       # Recent readings with sparkline
powermon sparkline                  # Sparkline of recent power draw
powermon today                      # Today's power statistics
powermon dump -n 50                 # Dump raw readings to terminal

# Historical views
powermon hourly -days 14            # Hourly aggregated bar chart
powermon daily -weeks 8             # Daily aggregated bar chart
```

### Output Examples

**Status:**
```
Power: 4.5 W
  CPU: 4.5 W  GPU: 0.0 W  Neural Accelerator (ANE): 0.0 W
Battery: 50%
Source: AC
Status: ⚡ Charging
Adapter: 60 W
```

**Live view:**
```
⚡  Total: 8.5 W
         System: 6.5 W  Battery: 2.0 W
         CPU: 5.0 W  GPU: 1.5 W  Neural Accelerator (ANE): 0.0 W
         Adapter: 60 W
  Battery: 51%  Source: AC
  Time: 14:05:00

  Power (W)
  ▁▂▃▅▆▇█▇▅▃▁
  ------------------------------------------------------------
  Min: 3.2  Avg: 5.8  Max: 12.3 W
  15 samples
```

**Dump:**
```
TIME                    TOTAL     CPU     GPU    NEURAL   BAT%  CHG      SYS    CHG W SOURCE
--------------------------------------------------------------------------------------------------------------
2026-04-20 14:00:00      5.2      4.7     0.5      0.0 W   50%    ⚡     5.2 W     0.0 W AC
2026-04-20 14:05:00      8.5      5.0     1.5      0.0 W   51%    ⚡     6.5 W     2.0 W AC
2026-04-20 14:10:00     12.3      8.0     2.1      0.4 W   52%    ⚡     8.3 W     4.0 W AC
```

## How It Works

### Power measurement
- **System power** (CPU + GPU + ANE): Read from `powermetrics --samplers cpu_power`
- **Neural Accelerator power**: Read directly from the `ANE Power:` line in `powermetrics` output when macOS exposes it
- **Battery charging power**: Estimated from battery % change rate × battery capacity
- **Adapter wattage**: Read from `system_profiler SPPowerDataType`

### Charging split
When the battery is charging, the tool estimates how much power goes to the system vs. charging the battery:

```
Charging power = (pct_change / 100) × battery_capacity_Wh / time_hours
System power = total_power - charging_power
```

The split updates whenever the battery % changes (typically every ~30 seconds during charging).

### Data storage
All readings are stored in SQLite at `~/.powermon/data/powermon.db`:

| Column | Description |
|--------|-------------|
| `timestamp` | Sample time |
| `power_draw` | Total CPU + GPU + ANE power (W) |
| `cpu_watts` | CPU power (W) |
| `gpu_watts` | GPU power (W) |
| `ank_watts` | Neural Accelerator / ANE power (W) |
| `charging_watts` | Power going to battery charging (W) |
| `system_watts` | Power going to system (W) |
| `is_charging` | Battery charging state (bool) |
| `battery_pct` | Battery percentage |
| `source` | AC or Battery |
| `adapter_w` | Charger rated wattage |

## Architecture

```
cmd/powermon/   CLI entry point with all commands
pkg/power/      macOS power data collection (powermetrics, pmset, system_profiler)
pkg/db/         SQLite storage layer via sqlite3 CLI
pkg/vis/        Terminal visualization (sparklines, bar charts)
```

## Requirements

- macOS (tested on Apple Silicon)
- Go 1.22+
- `sqlite3` CLI (pre-installed on macOS)
- Root privileges for full power metrics (`powermetrics`)

## License

MIT
