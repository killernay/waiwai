// Package throttle implements a token-bucket rate limiter for bandwidth control.
package throttle

import (
	"io"
	"sync"
	"time"
)

// Limiter is a thread-safe token bucket.
// Multiple goroutines (streams) share a single limiter so the total
// throughput across all streams stays within the configured rate.
type Limiter struct {
	rate     int64 // bytes per second, 0 = unlimited
	tokens   float64
	maxBurst float64
	lastFill time.Time
	mu       sync.Mutex
}

// New creates a Limiter. rate=0 means unlimited.
func New(bytesPerSec int64) *Limiter {
	burst := float64(bytesPerSec) * 0.1 // 100ms burst headroom
	if burst < float64(64*1024) {
		burst = float64(64 * 1024) // min 64 KB burst
	}
	return &Limiter{
		rate:     bytesPerSec,
		tokens:   burst,
		maxBurst: burst,
		lastFill: time.Now(),
	}
}

// Wait blocks until n bytes can be sent, then consumes the tokens.
func (l *Limiter) Wait(n int) {
	if l.rate == 0 {
		return // unlimited
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	for {
		l.refill()
		if l.tokens >= float64(n) {
			l.tokens -= float64(n)
			return
		}
		// Calculate how long to sleep for enough tokens
		need := float64(n) - l.tokens
		sleepSec := need / float64(l.rate)
		l.mu.Unlock()
		time.Sleep(time.Duration(sleepSec * float64(time.Second)))
		l.mu.Lock()
	}
}

func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastFill).Seconds()
	l.lastFill = now
	l.tokens += elapsed * float64(l.rate)
	if l.tokens > l.maxBurst {
		l.tokens = l.maxBurst
	}
}

// UpdateRate changes the rate at runtime (e.g. from monitoring endpoint).
func (l *Limiter) UpdateRate(bytesPerSec int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rate = bytesPerSec
	burst := float64(bytesPerSec) * 0.1
	if burst < float64(64*1024) {
		burst = float64(64 * 1024)
	}
	l.maxBurst = burst
}

// ─── ThrottledWriter wraps an io.Writer with rate limiting ───────────────────

type ThrottledWriter struct {
	w       io.Writer
	limiter *Limiter
}

func NewWriter(w io.Writer, limiter *Limiter) *ThrottledWriter {
	return &ThrottledWriter{w: w, limiter: limiter}
}

func (tw *ThrottledWriter) Write(p []byte) (int, error) {
	tw.limiter.Wait(len(p))
	return tw.w.Write(p)
}
