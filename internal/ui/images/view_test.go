package images

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
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
			{OS: "linux", Architecture: "arm64", Size: 5242880},
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
		"linux/arm64", "linux/amd64",
		"5 MiB", "2 MiB",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q", want)
		}
	}
}

func TestDetailView_inspectKeyedByRef(t *testing.T) {
	// Inspect belonged to a different image: the fields must not leak into the
	// currently selected one.
	m := New().SetItems([]backend.Image{
		{ID: "28bd5fe8b56d", Repository: "docker.io/library/alpine", Tag: "latest", Size: 3848024},
	}).SetInspect("other/image:v1", &backend.ImageInspect{Digest: "sha256:stale"}, nil)
	v := ansi.Strip(m.DetailView(60, 40))
	if strings.Contains(v, "Digest") || strings.Contains(v, "stale") {
		t.Fatalf("stale inspect rendered for a different ref: %q", v)
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
