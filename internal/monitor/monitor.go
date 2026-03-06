// Package monitor exposes real-time transfer metrics over HTTP.
// Both sender and receiver run a monitor; each side can scrape the other.
//
// Endpoints:
//   GET /metrics   Prometheus text format
//   GET /status    JSON snapshot (human-friendly)
package monitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds all counters. Fields are updated atomically by transfer goroutines.
type Metrics struct {
	StartTime     time.Time
	BytesSent     atomic.Int64
	BytesAcked    atomic.Int64
	ChunksSent    atomic.Int64
	ChunksAcked   atomic.Int64
	FilesComplete atomic.Int32
	FilesTotal    atomic.Int32
	Errors        atomic.Int32
	ActiveStreams  atomic.Int32

	// Computed on /status or /metrics call
	lastSnap     int64 // bytes at last interval
	lastSnapTime time.Time
	ThroughputMB float64 // MB/s rolling average
}

type Snapshot struct {
	UptimeSeconds float64 `json:"uptime_s"`
	BytesSent     int64   `json:"bytes_sent"`
	BytesAcked    int64   `json:"bytes_acked"`
	ThroughputMBs float64 `json:"throughput_mbs"`
	FilesComplete int32   `json:"files_complete"`
	FilesTotal    int32   `json:"files_total"`
	ActiveStreams  int32   `json:"active_streams"`
	Errors        int32   `json:"errors"`
	ETA           string  `json:"eta,omitempty"`
	TotalBytes    int64   `json:"total_bytes,omitempty"`
}

var Global = &Metrics{StartTime: time.Now()}

// Snapshot computes a point-in-time snapshot.
func (m *Metrics) Snapshot(totalBytes int64) Snapshot {
	now := time.Now()
	sent := m.BytesSent.Load()

	// Rolling throughput over last interval
	elapsed := now.Sub(m.lastSnapTime).Seconds()
	if elapsed >= 0.5 && m.lastSnapTime != (time.Time{}) {
		delta := sent - m.lastSnap
		m.ThroughputMB = float64(delta) / elapsed / 1e6
	}
	m.lastSnap = sent
	m.lastSnapTime = now

	snap := Snapshot{
		UptimeSeconds: now.Sub(m.StartTime).Seconds(),
		BytesSent:     sent,
		BytesAcked:    m.BytesAcked.Load(),
		ThroughputMBs: m.ThroughputMB,
		FilesComplete: m.FilesComplete.Load(),
		FilesTotal:    m.FilesTotal.Load(),
		ActiveStreams:  m.ActiveStreams.Load(),
		Errors:        m.Errors.Load(),
		TotalBytes:    totalBytes,
	}

	// ETA
	if m.ThroughputMB > 0 && totalBytes > 0 {
		remaining := float64(totalBytes-sent) / 1e6
		etaSec := remaining / m.ThroughputMB
		snap.ETA = fmt.Sprintf("%.0fs", etaSec)
	}
	return snap
}

// Serve starts the monitoring HTTP server on the given address.
func Serve(addr string, totalBytes int64) {
	mux := http.NewServeMux()

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Global.Snapshot(totalBytes))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		snap := Global.Snapshot(totalBytes)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, `# HELP qt_bytes_sent Total bytes sent
# TYPE qt_bytes_sent counter
qt_bytes_sent %d

# HELP qt_throughput_mbs Current throughput MB/s
# TYPE qt_throughput_mbs gauge
qt_throughput_mbs %.3f

# HELP qt_files_complete Files fully transferred
# TYPE qt_files_complete gauge
qt_files_complete %d

# HELP qt_active_streams Current parallel streams
# TYPE qt_active_streams gauge
qt_active_streams %d

# HELP qt_errors_total Total transfer errors
# TYPE qt_errors_total counter
qt_errors_total %d
`,
			snap.BytesSent,
			snap.ThroughputMBs,
			snap.FilesComplete,
			snap.ActiveStreams,
			snap.Errors,
		)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	go func() {
		_ = http.ListenAndServe(addr, mux)
	}()
}
