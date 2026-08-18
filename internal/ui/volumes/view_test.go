package volumes

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Laaaaksh/vessel/internal/backend"
)

func TestDetailView_noSelection(t *testing.T) {
	v := ansi.Strip(New().DetailView(60, 40))
	if !strings.Contains(v, "no volume selected") {
		t.Fatalf("expected empty state, got %q", v)
	}
}

func TestDetailView_rendersInspectFields(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		{Name: "vessel-test-vol", Driver: "local", Mountpoint: "/data/volume.img"},
	}).SetInspect("vessel-test-vol", &backend.VolumeInspect{
		Name:       "vessel-test-vol",
		Driver:     "local",
		Mountpoint: "/data/volume.img",
		SizeBytes:  549755813888,
		Format:     "ext4",
		Labels:     map[string]string{"purpose": "testing"},
		Options:    map[string]string{"sync": "fsync"},
	}, nil)
	v := ansi.Strip(m.DetailView(60, 40))
	for _, want := range []string{
		"vessel-test-vol",
		"Driver", "local",
		"Format", "ext4",
		"Size", "512 GiB",
		"-- Labels --", "purpose=testing",
		"-- Options --", "sync=fsync",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail missing %q", want)
		}
	}
}

func TestDetailView_inspectKeyedByName(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		{Name: "vessel-test-vol", Driver: "local"},
	}).SetInspect("other-vol", &backend.VolumeInspect{Name: "other-vol", Format: "ext4", SizeBytes: 1}, nil)
	v := ansi.Strip(m.DetailView(60, 40))
	// Base fields still render; the keyed inspect extras must not.
	if strings.Contains(v, "Size") || strings.Contains(v, "ext4") {
		t.Fatalf("stale inspect rendered for a different volume: %q", v)
	}
	if !strings.Contains(v, "Driver") {
		t.Fatalf("base detail fields missing: %q", v)
	}
}

func TestDetailView_inspectErrorRendered(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		{Name: "vessel-test-vol", Driver: "local"},
	}).SetInspect("vessel-test-vol", nil, errors.New("boom"))
	v := ansi.Strip(m.DetailView(60, 40))
	if !strings.Contains(v, "boom") {
		t.Fatalf("expected inspect error in detail, got %q", v)
	}
}

func TestListView_rendersRows(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		{Name: "vessel-test-vol", Driver: "local", SizeBytes: 549755813888},
	})
	v := ansi.Strip(m.ListView(80, 20))
	if !strings.Contains(v, "vessel-test-vol") {
		t.Fatalf("list missing volume: %q", v)
	}
	if !strings.Contains(v, "NAME") {
		t.Fatalf("list missing header: %q", v)
	}
}
