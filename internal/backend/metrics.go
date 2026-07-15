package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// cliMetrics is the raw JSON from `container stats --json`.
type cliMetrics struct {
	ID     string  `json:"id"`
	CPU    float64 `json:"cpuPercent"`
	MemUse uint64  `json:"memUsageBytes"`
	MemLim uint64  `json:"memLimitBytes"`
}

// MetricsSnapshot holds the latest metrics for all containers.
type MetricsSnapshot struct {
	mu   sync.RWMutex
	data map[string]Metrics // keyed by container ID
}

func newMetricsSnapshot() *MetricsSnapshot {
	return &MetricsSnapshot{data: make(map[string]Metrics)}
}

// Get returns metrics for a container ID, and whether it was found.
func (s *MetricsSnapshot) Get(id string) (Metrics, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.data[id]
	return m, ok
}

// set replaces the entire metrics map atomically.
func (s *MetricsSnapshot) set(data map[string]Metrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
}

// Poller fetches metrics on a fixed interval and stores them in a MetricsSnapshot.
type Poller struct {
	client   *Client
	snapshot *MetricsSnapshot
	interval time.Duration
}

// NewPoller creates a Poller with the given interval.
func NewPoller(client *Client, interval time.Duration) *Poller {
	return &Poller{
		client:   client,
		snapshot: newMetricsSnapshot(),
		interval: interval,
	}
}

// Snapshot returns the shared MetricsSnapshot.
func (p *Poller) Snapshot() *MetricsSnapshot { return p.snapshot }

// Run starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	pctx, cancel := context.WithTimeout(ctx, p.client.timeout)
	defer cancel()

	out, err := p.client.run(pctx, "stats", "--all", "--no-stream", "--json")
	if err != nil {
		// Metrics polling failures are non-fatal: the UI shows N/A.
		return
	}

	var raw []cliMetrics
	if err := json.Unmarshal(out, &raw); err != nil {
		return
	}

	data := make(map[string]Metrics, len(raw))
	for _, r := range raw {
		data[r.ID] = Metrics{
			ContainerID: r.ID,
			CPUPercent:  r.CPU,
			MemUsage:    r.MemUse,
			MemLimit:    r.MemLim,
		}
	}
	p.snapshot.set(data)
}

// FormatCPU formats CPU as a percentage string.
func FormatCPU(m Metrics, ok bool) string {
	if !ok {
		return "N/A"
	}
	return fmt.Sprintf("%.1f%%", m.CPUPercent)
}

// FormatMem formats memory as a human-readable string.
func FormatMem(m Metrics, ok bool) string {
	if !ok || m.MemLimit == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%s / %s", humanBytes(m.MemUsage), humanBytes(m.MemLimit))
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
