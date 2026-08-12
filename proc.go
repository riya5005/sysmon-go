package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CPUStats holds one snapshot of /proc/stat's aggregate "cpu" line.
// Linux exposes cumulative jiffie counters since boot, not a live
// percentage — so a real CPU % requires two snapshots and a delta,
// which is what CPUUsagePercent does below.
type CPUStats struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

func (c CPUStats) total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ + c.SoftIRQ + c.Steal
}

func (c CPUStats) idleTotal() uint64 {
	return c.Idle + c.IOWait
}

// readCPUStats parses the first "cpu" summary line of /proc/stat.
func readCPUStats() (CPUStats, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return CPUStats{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		// fields[0] == "cpu", the rest are the counters in order.
		vals := make([]uint64, 8)
		for i := 1; i < len(fields) && i <= 8; i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			vals[i-1] = v
		}
		return CPUStats{
			User: vals[0], Nice: vals[1], System: vals[2], Idle: vals[3],
			IOWait: vals[4], IRQ: vals[5], SoftIRQ: vals[6], Steal: vals[7],
		}, nil
	}
	return CPUStats{}, fmt.Errorf("cpu line not found in /proc/stat")
}

// CPUUsagePercent takes two snapshots ~sampleWindow apart and returns
// the percentage of non-idle time between them. This mirrors how
// tools like top/htop actually compute "current" CPU usage.
func CPUUsagePercent(before, after CPUStats) float64 {
	totalDelta := float64(after.total() - before.total())
	idleDelta := float64(after.idleTotal() - before.idleTotal())
	if totalDelta <= 0 {
		return 0
	}
	return (1.0 - idleDelta/totalDelta) * 100
}

// MemoryStats mirrors the key fields of /proc/meminfo, in kB as Linux reports them.
type MemoryStats struct {
	TotalKB     uint64  `json:"total_kb"`
	FreeKB      uint64  `json:"free_kb"`
	AvailableKB uint64  `json:"available_kb"`
	UsedKB      uint64  `json:"used_kb"`
	UsedPercent float64 `json:"used_percent"`
}

func readMemoryStats() (MemoryStats, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryStats{}, err
	}
	defer f.Close()

	fields := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, _ := strconv.ParseUint(parts[1], 10, 64)
		fields[key] = val
	}

	m := MemoryStats{
		TotalKB:     fields["MemTotal"],
		FreeKB:      fields["MemFree"],
		AvailableKB: fields["MemAvailable"],
	}
	m.UsedKB = m.TotalKB - m.AvailableKB
	if m.TotalKB > 0 {
		m.UsedPercent = float64(m.UsedKB) / float64(m.TotalKB) * 100
	}
	return m, nil
}

// LoadAvg mirrors /proc/loadavg's 1/5/15 minute load averages.
type LoadAvg struct {
	Load1  float64 `json:"load_1min"`
	Load5  float64 `json:"load_5min"`
	Load15 float64 `json:"load_15min"`
}

func readLoadAvg() (LoadAvg, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadAvg{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return LoadAvg{}, fmt.Errorf("unexpected /proc/loadavg format")
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return LoadAvg{Load1: l1, Load5: l5, Load15: l15}, nil
}

// ProcessInfo is a minimal summary of one running process, read from
// /proc/<pid>/comm and /proc/<pid>/status.
type ProcessInfo struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Memory string `json:"memory_kb"`
}

// listProcesses walks /proc and reads basic info for every numeric
// directory (each one is a PID). Processes that exit mid-scan are
// skipped rather than treated as errors — this is normal on a live system.
func listProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var procs []ProcessInfo
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // not a PID directory (e.g. "self", "net", "cpuinfo")
		}

		nameBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue // process likely exited between ReadDir and now
		}

		state, memKB := readProcessStatus(pid)

		procs = append(procs, ProcessInfo{
			PID:    pid,
			Name:   strings.TrimSpace(string(nameBytes)),
			State:  state,
			Memory: memKB,
		})
	}
	return procs, nil
}

func readProcessStatus(pid int) (state string, vmRSSkb string) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "unknown", "0"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "State:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				state = fields[1]
			}
		}
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				vmRSSkb = fields[1]
			}
		}
	}
	if state == "" {
		state = "unknown"
	}
	if vmRSSkb == "" {
		vmRSSkb = "0"
	}
	return state, vmRSSkb
}
