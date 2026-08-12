package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/stats/cpu", cpuHandler)
	mux.HandleFunc("/stats/memory", memoryHandler)
	mux.HandleFunc("/stats/load", loadHandler)
	mux.HandleFunc("/stats/processes", processesHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	addr := ":8080"
	log.Printf("sysmon listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

// cpuHandler takes two /proc/stat snapshots 200ms apart to compute a
// live usage percentage, rather than a raw cumulative counter.
func cpuHandler(w http.ResponseWriter, r *http.Request) {
	before, err := readCPUStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	time.Sleep(200 * time.Millisecond)
	after, err := readCPUStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"usage_percent": round2(CPUUsagePercent(before, after)),
	})
}

func memoryHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := readMemoryStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func loadHandler(w http.ResponseWriter, r *http.Request) {
	load, err := readLoadAvg()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, load)
}

func processesHandler(w http.ResponseWriter, r *http.Request) {
	procs, err := listProcesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"count":     len(procs),
		"processes": procs,
	})
}

// metricsHandler emits stats in Prometheus plaintext exposition format,
// so this could be scraped by a real Prometheus/OpenShift monitoring
// stack without any extra library.
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	mem, err := readMemoryStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	load, err := readLoadAvg()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP sysmon_memory_used_percent Percentage of memory used\n")
	fmt.Fprintf(w, "# TYPE sysmon_memory_used_percent gauge\n")
	fmt.Fprintf(w, "sysmon_memory_used_percent %.2f\n", mem.UsedPercent)

	fmt.Fprintf(w, "# HELP sysmon_load1 1-minute load average\n")
	fmt.Fprintf(w, "# TYPE sysmon_load1 gauge\n")
	fmt.Fprintf(w, "sysmon_load1 %.2f\n", load.Load1)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func round2(f float64) float64 {
	return float64(int(f*100)) / 100
}
