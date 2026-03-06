// Package ui provides a terminal progress display for multi-file transfers.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const barWidth = 30

// FileProgress tracks progress for a single file.
type FileProgress struct {
	Name     string
	Total    int64
	done     int64
	started  time.Time
	mu       sync.Mutex
}

func (f *FileProgress) Add(n int64) {
	f.mu.Lock()
	f.done += n
	f.mu.Unlock()
}

func (f *FileProgress) Done() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

// Display renders a multi-file progress UI to the terminal.
type Display struct {
	files    []*FileProgress
	total    int64
	start    time.Time
	mu       sync.Mutex
	out      io.Writer
	lines    int // lines printed last frame (for cursor reset)
	ticker   *time.Ticker
	stopCh   chan struct{}
}

func New(out io.Writer) *Display {
	if out == nil {
		out = os.Stderr
	}
	return &Display{out: out, start: time.Now(), stopCh: make(chan struct{})}
}

// AddFile registers a file to track and returns its progress handle.
func (d *Display) AddFile(name string, size int64) *FileProgress {
	d.mu.Lock()
	defer d.mu.Unlock()
	fp := &FileProgress{Name: name, Total: size, started: time.Now()}
	d.files = append(d.files, fp)
	d.total += size
	return fp
}

// Start begins the render loop (call once).
func (d *Display) Start() {
	d.ticker = time.NewTicker(200 * time.Millisecond)
	go func() {
		for {
			select {
			case <-d.ticker.C:
				d.render()
			case <-d.stopCh:
				d.render() // final frame
				return
			}
		}
	}()
}

// Stop halts the display.
func (d *Display) Stop() {
	d.ticker.Stop()
	close(d.stopCh)
	time.Sleep(250 * time.Millisecond)
	fmt.Fprintln(d.out)
}

func (d *Display) render() {
	d.mu.Lock()
	files := make([]*FileProgress, len(d.files))
	copy(files, d.files)
	d.mu.Unlock()

	// Reset cursor to top of our block
	if d.lines > 0 {
		fmt.Fprintf(d.out, "\033[%dA\033[J", d.lines)
	}

	var totalDone int64
	var sb strings.Builder

	for _, f := range files {
		done := f.Done()
		totalDone += done
		pct := float64(done) / float64(f.Total)
		if pct > 1 {
			pct = 1
		}

		filled := int(pct * barWidth)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		elapsed := time.Since(f.started).Seconds()
		var speed string
		if elapsed > 0 {
			mbps := float64(done) / elapsed / 1e6
			speed = fmt.Sprintf("%.1f MB/s", mbps)
		}

		name := truncate(f.Name, 24)
		fmt.Fprintf(&sb, "  %-24s [%s] %5.1f%%  %s\n",
			name, bar, pct*100, speed)
	}

	// Overall progress bar
	totalPct := 0.0
	if d.total > 0 {
		totalPct = float64(totalDone) / float64(d.total)
	}
	elapsed := time.Since(d.start).Seconds()
	overallSpeed := 0.0
	if elapsed > 0 {
		overallSpeed = float64(totalDone) / elapsed / 1e6
	}
	eta := ""
	if overallSpeed > 0 && d.total > totalDone {
		etaSec := float64(d.total-totalDone) / overallSpeed / 1e6
		eta = fmt.Sprintf(" ETA %ds", int(etaSec))
	}

	filled := int(totalPct * barWidth)
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Fprintf(&sb, "\n  TOTAL  [%s] %5.1f%%  %.1f MB/s%s\n",
		bar, totalPct*100, overallSpeed, eta)

	output := sb.String()
	fmt.Fprint(d.out, output)
	d.lines = strings.Count(output, "\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-(n-1):]
}
