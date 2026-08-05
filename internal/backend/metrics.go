package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Apple container stats JSON (container 1.2.x).
type cliMetrics struct {
	ID               string `json:"id"`
	CPUUsageUsec     uint64 `json:"cpuUsageUsec"`
	MemoryUsageBytes uint64 `json:"memoryUsageBytes"`
	MemoryLimitBytes uint64 `json:"memoryLimitBytes"`
}

type cpuSample struct {
	usec uint64
	at   time.Time
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

func (s *MetricsSnapshot) set(data map[string]Metrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
}

const historyLen = 30

// Poller fetches metrics on a fixed interval and stores them in a MetricsSnapshot.
type Poller struct {
	client   *Client
	snapshot *MetricsSnapshot
	interval time.Duration

	prevMu sync.Mutex
	prev   map[string]cpuSample

	histMu  sync.RWMutex
	history map[string][]float64 // CPU% ring samples
}

// NewPoller creates a Poller with the given interval.
func NewPoller(client *Client, interval time.Duration) *Poller {
	return &Poller{
		client:   client,
		snapshot: newMetricsSnapshot(),
		interval: interval,
		prev:     make(map[string]cpuSample),
		history:  make(map[string][]float64),
	}
}

// Sparkline returns a compact unicode sparkline of recent CPU samples.
func (p *Poller) Sparkline(id string, width int) string {
	if width < 4 {
		return ""
	}
	p.histMu.RLock()
	samples := append([]float64{}, p.history[id]...)
	p.histMu.RUnlock()
	if len(samples) == 0 {
		return ""
	}
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range samples {
		idx := int(v / 12.5) // 0..7 for 0..100%
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		b.WriteRune(bars[idx])
	}
	return b.String()
}

// Snapshot returns the shared MetricsSnapshot.
func (p *Poller) Snapshot() *MetricsSnapshot { return p.snapshot }

// Run starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	p.poll(ctx)
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

	out, err := p.client.run(pctx, "stats", "--no-stream", "--format", "json")
	if err != nil {
		return
	}

	var raw []cliMetrics
	if err := json.Unmarshal(out, &raw); err != nil {
		return
	}

	now := time.Now()
	data := make(map[string]Metrics, len(raw))
	// Rebuilt from scratch so samples for removed containers do not accumulate.
	next := make(map[string]cpuSample, len(raw))

	p.prevMu.Lock()
	defer p.prevMu.Unlock()

	for _, r := range raw {
		m := Metrics{
			ContainerID: r.ID,
			MemUsage:    r.MemoryUsageBytes,
			MemLimit:    r.MemoryLimitBytes,
		}
		if prev, ok := p.prev[r.ID]; ok && now.After(prev.at) && r.CPUUsageUsec >= prev.usec {
			deltaCPU := float64(r.CPUUsageUsec - prev.usec)
			deltaWall := now.Sub(prev.at).Seconds() * 1e6 // wall µs
			if deltaWall > 0 {
				m.CPUPercent = (deltaCPU / deltaWall) * 100
			}
		}
		next[r.ID] = cpuSample{usec: r.CPUUsageUsec, at: now}
		data[r.ID] = m
		p.pushHistory(r.ID, m.CPUPercent)
	}
	p.prev = next
	p.snapshot.set(data)
}

func (p *Poller) pushHistory(id string, cpu float64) {
	p.histMu.Lock()
	defer p.histMu.Unlock()
	h := append(p.history[id], cpu)
	if len(h) > historyLen {
		h = h[len(h)-historyLen:]
	}
	p.history[id] = h
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
