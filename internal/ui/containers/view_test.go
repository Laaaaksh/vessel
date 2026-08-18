package containers

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

// column returns the rune offset of sub within s, ignoring styling escapes.
func column(t *testing.T, s, sub string) int {
	t.Helper()
	plain := ansi.Strip(s)
	i := strings.Index(plain, sub)
	if i < 0 {
		t.Fatalf("%q not found in %q", sub, plain)
	}
	return utf8.RuneCountInString(plain[:i])
}

// Names shorter than colName must still fill the column, otherwise every data
// cell after NAME slides left and stops lining up with its header.
func TestRenderRowAlignsWithHeader(t *testing.T) {
	const width = 100
	m := New().SetItems([]backend.Container{
		{ID: "a", Name: "short", Status: "running"},
		{ID: "b", Name: "a-much-longer-container-name-that-overflows", Status: "stopped"},
	})

	header := m.renderHeader(width)
	wantStatus := column(t, header, "STATUS")

	for i, c := range m.filtered {
		row := m.renderRow(c, false, width, nil)
		if got := column(t, row, c.Status); got != wantStatus {
			t.Errorf("row %d (%s): status at column %d, header STATUS at %d", i, c.Name, got, wantStatus)
		}
	}
}

// TestDetailView_rendersInspectDepth checks that the detail pane surfaces the
// fields the list JSON already carries: network/IP, mounts, resources,
// platform, hostname.
func TestDetailView_rendersInspectDepth(t *testing.T) {
	m := New().SetItems([]backend.Container{{
		ID:          "abc",
		Name:        "web",
		Status:      "running",
		Hostname:    "web",
		Platform:    "linux/arm64",
		CPUs:        4,
		MemoryBytes: 1073741824,
		Mounts:      []backend.Mount{{Source: "/host/data", Destination: "/data"}},
		Networks:    []backend.Network{{Name: "default", IP: "192.168.64.2/24"}},
	}})
	v := ansi.Strip(m.DetailView(60, 40, nil))
	for _, want := range []string{
		"web",
		"Hostname", "web",
		"Platform", "linux/arm64",
		"CPUs", "4",
		"Memory", "1 GiB",
		"-- Mounts --", "/host/data → /data",
		"Networks", "default (192.168.64.2/24)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q", want)
		}
	}
}

func TestDetailView_noSelection(t *testing.T) {
	v := ansi.Strip(New().DetailView(60, 40, nil))
	if !strings.Contains(v, "no container selected") {
		t.Fatalf("expected empty state, got %q", v)
	}
}

func TestDetailView_staysWithinHeightBudgetAndKeepsMetrics(t *testing.T) {
	mounts := make([]backend.Mount, 12)
	for i := range mounts {
		mounts[i] = backend.Mount{Source: fmt.Sprintf("/host/data-%d", i), Destination: fmt.Sprintf("/data-%d", i)}
	}
	labels := map[string]string{"a": "1", "b": "2"}
	m := New().SetItems([]backend.Container{{
		ID: "abc", Name: "web", Status: "running",
		Mounts: mounts, Labels: labels,
		Env: []string{"PATH=/usr/bin", "HOME=/root"},
	}})
	poller := backend.NewPoller(nil, time.Second)

	const height = 20
	v := ansi.Strip(m.DetailView(60, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into a %d-line budget", got, height)
	}
	lines := strings.Split(v, "\n")
	for i, l := range lines {
		head := strings.TrimSpace(l)
		if !strings.HasPrefix(head, "--") {
			continue
		}
		next := ""
		if i+1 < len(lines) {
			next = strings.TrimSpace(lines[i+1])
		}
		if next == "" || strings.HasPrefix(next, "--") {
			t.Errorf("section header %q rendered with no rows under it", head)
		}
	}
	// The live metrics block must not be crowded out by a long mounts list.
	if !strings.Contains(v, "CPU") {
		t.Errorf("live metrics squeezed out by the mounts section: %q", v)
	}
}

func TestDetailView_fitsTheMinimumTerminalSize(t *testing.T) {
	m := New().SetItems([]backend.Container{{
		ID:          "abc",
		Name:        "web",
		Status:      "running",
		Hostname:    "web",
		Platform:    "linux/arm64",
		CPUs:        4,
		MemoryBytes: 1073741824,
		Mounts:      []backend.Mount{{Source: "/host/data", Destination: "/data"}},
		Networks:    []backend.Network{{Name: "default", IP: "192.168.64.2/24"}},
		Labels:      map[string]string{"a": "1"},
		Env:         []string{"PATH=/usr/bin"},
	}})
	poller := backend.NewPoller(nil, time.Second)

	// A 60-col terminal, the narrowest the app accepts, gives the detail pane
	// 18 columns, so rows wrap as well as run long.
	for _, width := range []int{18, 38} {
		for _, height := range []int{4, 6, 8, 10} {
			v := ansi.Strip(m.DetailView(width, height, poller))
			if got := strings.Count(v, "\n") + 1; got > height {
				t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
			}
		}
	}
}

func TestDetailView_narrowPaneRendersRowsInOrder(t *testing.T) {
	m := New().SetItems([]backend.Container{{
		ID:       "abc123456789",
		Name:     "web",
		Image:    "alpine:latest",
		Status:   "running",
		Hostname: "very-long-hostname-value",
		Platform: "arm64",
	}})
	poller := backend.NewPoller(nil, time.Second)

	// The Hostname row wraps to two rows and does not fit, while the
	// single-row Platform row after it would fit in the gap it left behind.
	const width, height = 20, 11
	v := ansi.Strip(m.DetailView(width, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	if !strings.Contains(v, "Ports") {
		t.Fatalf("expected the rows before Hostname to render, so the gap is real: %q", v)
	}
	if strings.Contains(v, "Platform") && !strings.Contains(v, "Hostname") {
		t.Errorf("Platform rendered while the earlier Hostname row was dropped: %q", v)
	}
}
