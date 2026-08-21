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
		"MemLimit", "1 GiB",
		"-- Mounts --", "/host/data → /data",
		"Networks", "default (192.168.64.2/24)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q", want)
		}
	}
}

// The configured memory limit and the poller's live memory usage are two
// different numbers and render together whenever a poller is attached, so they
// must not share a label. Only the live row is called "Memory".
func TestDetailView_limitAndLiveMemoryRowsAreDistinct(t *testing.T) {
	m := New().SetItems([]backend.Container{{
		ID: "abc", Name: "web", Status: "running", MemoryBytes: 1073741824,
	}})
	poller := backend.NewPoller(nil, time.Second)

	v := ansi.Strip(m.DetailView(60, 40, poller))

	if !strings.Contains(v, "MemLimit: 1 GiB") {
		t.Errorf("configured memory limit row missing: %q", v)
	}
	if !strings.Contains(v, "Memory:") {
		t.Errorf("live memory row missing: %q", v)
	}
	if got := strings.Count(v, "Memory:"); got != 1 {
		t.Errorf("%d rows labelled \"Memory:\"; the limit and live usage must be told apart: %q", got, v)
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

	// Pinned so the invariant is actually exercised: at 20x6 the pane has room
	// for the title, its spacer and Image, then the ID row wraps to two rows
	// and no longer fits in the single row left. The single-row Status row
	// after it would fit in that gap, and must be dropped anyway so what
	// renders stays a prefix of what was asked for.
	const width, height = 20, 6
	v := ansi.Strip(m.DetailView(width, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	if !strings.Contains(v, "Image") {
		t.Fatalf("the rows before the gap must render: %q", v)
	}
	if strings.Contains(v, "ID") {
		t.Fatalf("geometry drifted: the wrapping ID row was expected not to fit: %q", v)
	}
	for _, dropped := range []string{"Status", "Ports", "Hostname", "Platform"} {
		if strings.Contains(v, dropped) {
			t.Errorf("%s jumped ahead of the dropped ID row: %q", dropped, v)
		}
	}
}

func TestDetailView_uuidHostnameDoesNotEvictLaterRows(t *testing.T) {
	// A container started without --name gets its UUID as the hostname, which
	// is 36 characters: unbounded next to the 9-column label, so the row must
	// be width-bound like its siblings or it wraps and evicts the rows after it.
	m := New().SetItems([]backend.Container{{
		ID: "abc123456789", Name: "web", Image: "alpine:latest", Status: "running",
		Ports:       []backend.PortMapping{{HostPort: 8080, ContainerPort: 80}},
		Hostname:    "a5337e3a-3096-4b1a-b698-96aaa10814b1",
		Platform:    "linux/arm64",
		CPUs:        4,
		MemoryBytes: 1073741824,
		Networks:    []backend.Network{{Name: "default", IP: "192.168.64.2/24"}},
		Mounts:      []backend.Mount{{Source: "p2-live-probe", Destination: "/data"}},
	}})
	poller := backend.NewPoller(nil, time.Second)

	// The detail pane on a real terminal is 40 columns wide.
	const width, height = 40, 16
	v := ansi.Strip(m.DetailView(width, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	if !strings.Contains(v, "Hostname") {
		t.Fatalf("the hostname row itself must still render: %q", v)
	}
	for _, want := range []string{"Platform", "CPUs", "MemLimit", "Networks", "-- Mounts --"} {
		if !strings.Contains(v, want) {
			t.Errorf("%q evicted by an unbounded Hostname row: %q", want, v)
		}
	}
}

func TestDetailView_longImageRefDoesNotEvictLaterRows(t *testing.T) {
	// The Image row is a key/value row like its siblings and must be bound to
	// one rendered row the same way, or a long reference wraps and evicts
	// every row after it at minimum terminal geometry.
	m := New().SetItems([]backend.Container{{
		ID:       "abc123456789",
		Name:     "web",
		Image:    "docker.io/library/postgres:16.2-alpine",
		Status:   "running",
		Ports:    []backend.PortMapping{{HostPort: 5432, ContainerPort: 5432}},
		Hostname: "db",
		Platform: "linux/arm64",
	}})
	poller := backend.NewPoller(nil, time.Second)

	// The narrowest pane geometry the app accepts (60x12 terminal).
	const width, height = 18, 10
	v := ansi.Strip(m.DetailView(width, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	if !strings.Contains(v, "Status") {
		t.Errorf("Status row evicted by an unbounded Image row: %q", v)
	}
}

func TestDetailView_manyPortsDoNotEvictLaterRows(t *testing.T) {
	// The Ports row is a key/value row like its siblings and must be bound to
	// one rendered row the same way, or a long published-port list wraps over
	// most of the pane and evicts every row after it.
	ports := make([]backend.PortMapping, 15)
	for i := range ports {
		ports[i] = backend.PortMapping{HostPort: 18000 + i, ContainerPort: 8000 + i, Protocol: "tcp"}
	}
	m := New().SetItems([]backend.Container{{
		ID: "abc123456789", Name: "web", Image: "alpine:latest", Status: "running",
		Ports:    ports,
		Hostname: "web",
		Networks: []backend.Network{{Name: "default", IP: "192.168.64.2/24"}},
		Mounts:   []backend.Mount{{Source: "data", Destination: "/data"}},
	}})
	poller := backend.NewPoller(nil, time.Second)

	const width, height = 40, 14
	v := ansi.Strip(m.DetailView(width, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	for _, want := range []string{"Hostname", "Networks", "-- Mounts --"} {
		if !strings.Contains(v, want) {
			t.Errorf("%q evicted by an unbounded Ports row: %q", want, v)
		}
	}
}

func TestDetailView_sectionsDoNotJumpAheadOfDroppedRows(t *testing.T) {
	// A long image reference fills the pane before the identity rows are in;
	// the live metrics still render because room was reserved for them, but a
	// Labels section must not appear above rows that were dropped for space.
	m := New().SetItems([]backend.Container{{
		ID:     "abc123456789",
		Name:   "web",
		Image:  "docker.io/library/postgres:16.2alpine",
		Status: "running",
		Labels: map[string]string{"app": "db"},
	}})
	poller := backend.NewPoller(nil, time.Second)

	const width, height = 18, 7
	v := ansi.Strip(m.DetailView(width, height, poller))

	if got := strings.Count(v, "\n") + 1; got > height {
		t.Errorf("pane rendered %d lines into %dx%d", got, width, height)
	}
	// Pinned: the wrapping ID row fills the pane exactly, so Status is the
	// first row dropped and everything after it must stay dropped too.
	if !strings.Contains(v, "ID") {
		t.Fatalf("the identity rows before the gap must render: %q", v)
	}
	if strings.Contains(v, "Status") {
		t.Fatalf("geometry drifted: the Status row was expected not to fit: %q", v)
	}
	if !strings.Contains(v, "CPU") {
		t.Errorf("the reserved metrics rows must still render: %q", v)
	}
	if strings.Contains(v, "-- Labels --") {
		t.Errorf("Labels jumped ahead of the dropped Ports row: %q", v)
	}
}

// The Networks row is gated on len(sel.Networks) > 0, so a stopped container
// whose network survives mapping must actually reach the pane.
func TestDetailView_stoppedContainerRendersItsNetwork(t *testing.T) {
	m := New().SetItems([]backend.Container{{
		ID: "abc", Name: "web", Status: "stopped",
		Networks: []backend.Network{{Name: "default"}},
	}})

	v := ansi.Strip(m.DetailView(60, 40, nil))
	if !strings.Contains(v, "Networks: default") {
		t.Errorf("stopped container dropped its network row: %q", v)
	}
}
