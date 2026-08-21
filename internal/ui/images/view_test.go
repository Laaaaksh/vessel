package images

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
	"github.com/Laaaaksh/vessel/internal/ui/uiutil"
)

const testRef = "docker.io/library/alpine:latest"

func inspectModel() Model {
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	})
	return m.SetInspect(testRef, &backend.ImageInspect{
		ID:         "28bd5fe8b56d",
		Repository: "docker.io/library/alpine",
		Tag:        "latest",
		Digest:     "sha256:e7a1a92a5bfeee40966aea60f0796b0e",
		Size:       3848024,
		Cmd:        []string{"/bin/sh"},
		WorkingDir: "/",
		LayerCount: 1,
		Env:        []string{"PATH=/usr/local/sbin:/usr/local/bin"},
		Platforms: []backend.ImagePlatform{
			{OS: "linux", Architecture: "arm64", Variant: "v8", Size: 5242880},
			{OS: "linux", Architecture: "amd64", Size: 2097152},
		},
	}, nil)
}

func TestDetailView_noSelection(t *testing.T) {
	v := ansi.Strip(New().DetailView(60, 40))
	if !strings.Contains(v, "no image selected") {
		t.Fatalf("expected empty state, got %q", v)
	}
}

func TestDetailView_rendersInspectFields(t *testing.T) {
	m := inspectModel()
	v := ansi.Strip(m.DetailView(60, 40))
	for _, want := range []string{
		testRef,
		"Digest", "sha256:e7a1a92a5b",
		"Cmd", "/bin/sh",
		"Workdir", "/",
		"Layers", "1",
		"-- Env --", "PATH=/usr/local/sbin",
		"-- Platforms --",
		"linux/arm64/v8", "linux/amd64",
		"5 MiB", "2 MiB",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q", want)
		}
	}
}

func TestDetailView_inspectKeyedByRef(t *testing.T) {
	// The cached inspect belongs to the first image; after moving to the second
	// none of its fields may leak into that image's pane.
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
		{ID: "nginxid", Repository: "nginx", Tag: "1.27", Size: 1048576},
	}).SetInspect(testRef, &backend.ImageInspect{
		ID:         "28bd5fe8b56d",
		Digest:     "sha256:alpineonly",
		Cmd:        []string{"/bin/sh"},
		LayerCount: 1,
	}, nil)

	if v := ansi.Strip(m.DetailView(60, 40)); !strings.Contains(v, "sha256:alpineonly") {
		t.Fatalf("inspect not rendered for the image it belongs to: %q", v)
	}

	v := ansi.Strip(m.MoveBy(1).DetailView(60, 40))
	if !strings.Contains(v, "nginx:1.27") {
		t.Fatalf("expected the second image to be selected: %q", v)
	}
	for _, leaked := range []string{"sha256:alpineonly", "Digest", "/bin/sh", "Layers"} {
		if strings.Contains(v, leaked) {
			t.Errorf("previous image's inspect leaked into the next selection: %q in %q", leaked, v)
		}
	}
}

func TestDetailView_inspectErrorKeyedByRef(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest"},
		{ID: "nginxid", Repository: "nginx", Tag: "1.27"},
	}).SetInspect(testRef, nil, errors.New("boom"))

	v := ansi.Strip(m.MoveBy(1).DetailView(60, 40))
	if strings.Contains(v, "boom") {
		t.Errorf("another image's inspect error rendered: %q", v)
	}
}

func TestDetailView_inspectErrorRendered(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect(testRef, nil, errors.New("boom"))
	v := ansi.Strip(m.DetailView(60, 40))
	if !strings.Contains(v, "boom") {
		t.Fatalf("expected inspect error in detail, got %q", v)
	}
}

func TestListView_rendersRows(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "abc123", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	})
	v := ansi.Strip(m.ListView(80, 20))
	if !strings.Contains(v, "docker.io/library/alpine:latest") {
		t.Fatalf("list missing ref: %q", v)
	}
	if !strings.Contains(v, "REPOSITORY:TAG") {
		t.Fatalf("list missing header: %q", v)
	}
}

// A long inspect error must be indented and shortened to a single rendered row
// like every other section row, or it costs the pane extra rows it has already
// budgeted away.
func TestDetailView_longInspectErrorOccupiesOneRow(t *testing.T) {
	long := errors.New("inspect image docker.io/library/alpine:latest: exit status 1: unable to reach the container daemon")
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect(testRef, nil, long)

	for _, width := range []int{18, 40, 60} {
		v := ansi.Strip(m.DetailView(width, 20))
		row := lineContaining(v, "  inspect")
		if row == "" {
			t.Fatalf("no error row rendered at width %d: %q", width, v)
		}
		if !strings.HasPrefix(row, "  ") {
			t.Errorf("error row not indented at width %d: %q", width, row)
		}
		if got := uiutil.RowsFor(row, width); got != 1 {
			t.Errorf("error row occupies %d rendered rows at width %d, want 1: %q", got, width, row)
		}
	}
}

// The alpine index carries linux/arm twice, as v6 and v7. Without the variant
// both rows read as plain "linux/arm" and nothing tells them apart.
func TestDetailView_platformRowsDistinguishVariants(t *testing.T) {
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect(testRef, &backend.ImageInspect{
		ID:         "28bd5fe8b56d",
		Repository: "docker.io/library/alpine",
		Tag:        "latest",
		Platforms: []backend.ImagePlatform{
			{OS: "linux", Architecture: "arm", Variant: "v6", Size: 3555096},
			{OS: "linux", Architecture: "arm", Variant: "v7", Size: 3262261},
			{OS: "linux", Architecture: "amd64", Size: 3848024},
		},
	}, nil)

	v := ansi.Strip(m.DetailView(60, 40))
	for _, want := range []string{"linux/arm/v6", "linux/arm/v7"} {
		if !strings.Contains(v, want) {
			t.Errorf("platform row %q missing: %q", want, v)
		}
	}
	// A platform with no variant keeps the plain two-segment form.
	if !strings.Contains(v, "linux/amd64  ") {
		t.Errorf("variantless platform row lost its plain form: %q", v)
	}
	var rows []string
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "linux/arm") {
			rows = append(rows, strings.TrimSpace(l))
		}
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 linux/arm rows, got %v", rows)
	}
	if rows[0] == rows[1] {
		t.Errorf("the two linux/arm variants render identically: %q", rows[0])
	}
}
