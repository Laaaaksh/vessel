package containers

import (
	"strings"
	"testing"
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
