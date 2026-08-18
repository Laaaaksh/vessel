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
	// The cached inspect belongs to the first volume; after moving to the second
	// none of its fields may leak into that volume's pane.
	m := New().SetItems([]backend.Volume{
		{Name: "vessel-test-vol", Driver: "local"},
		{Name: "other-vol", Driver: "local"},
	}).SetInspect("vessel-test-vol", &backend.VolumeInspect{
		Name:      "vessel-test-vol",
		Format:    "ext4",
		SizeBytes: 549755813888,
		Labels:    map[string]string{"purpose": "testing"},
	}, nil)

	if v := ansi.Strip(m.DetailView(60, 40)); !strings.Contains(v, "ext4") {
		t.Fatalf("inspect not rendered for the volume it belongs to: %q", v)
	}

	v := ansi.Strip(m.MoveBy(1).DetailView(60, 40))
	if !strings.Contains(v, "other-vol") {
		t.Fatalf("expected the second volume to be selected: %q", v)
	}
	for _, leaked := range []string{"ext4", "512 GiB", "-- Labels --", "purpose=testing"} {
		if strings.Contains(v, leaked) {
			t.Errorf("previous volume's inspect leaked into the next selection: %q in %q", leaked, v)
		}
	}
	if !strings.Contains(v, "Driver") {
		t.Fatalf("base detail fields missing: %q", v)
	}
}

func TestDetailView_inspectErrorKeyedByName(t *testing.T) {
	m := New().SetItems([]backend.Volume{
		{Name: "vessel-test-vol", Driver: "local"},
		{Name: "other-vol", Driver: "local"},
	}).SetInspect("vessel-test-vol", nil, errors.New("boom"))

	v := ansi.Strip(m.MoveBy(1).DetailView(60, 40))
	if strings.Contains(v, "boom") {
		t.Errorf("another volume's inspect error rendered: %q", v)
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
