package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return v
}

func TestMapContainers_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliContainer](t, "list.json")
	got := mapContainers(raw)
	if len(got) == 0 {
		t.Fatal("expected at least one container from list.json")
	}
	found := false
	for _, c := range got {
		if c.Name == "vessel-probe" || c.ID == "vessel-probe" {
			found = true
			if c.Status != "running" {
				t.Errorf("vessel-probe status: want running, got %s", c.Status)
			}
			if c.Image == "" {
				t.Error("vessel-probe image empty")
			}
			if c.Created.IsZero() {
				t.Error("vessel-probe created empty")
			}
		}
		if c.Name == "vessel-ports" {
			if len(c.Ports) == 0 {
				t.Error("vessel-ports should have published ports")
			} else if c.Ports[0].HostPort != 8080 || c.Ports[0].ContainerPort != 80 {
				t.Errorf("vessel-ports ports wrong: %+v", c.Ports[0])
			}
			if c.Ports[0].Protocol != "tcp" {
				t.Errorf("protocol want tcp, got %s", c.Ports[0].Protocol)
			}
		}
	}
	if !found {
		t.Error("vessel-probe not found in mapped containers")
	}
}

func TestMapContainers_empty(t *testing.T) {
	got := mapContainers(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestMapImages_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliImage](t, "images.json")
	got := mapImages(raw)
	if len(got) == 0 {
		t.Fatal("expected images")
	}
	img := got[0]
	if img.Repository == "" {
		t.Error("repository empty")
	}
	if img.ID == "" {
		t.Error("id empty")
	}
	ref := FormatRef(img)
	if ref == "" {
		t.Error("FormatRef empty")
	}
}

func TestMapVolumes_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliVolume](t, "volumes.json")
	got := mapVolumes(raw)
	if len(got) == 0 {
		t.Fatal("expected volumes")
	}
	if got[0].Name == "" {
		t.Error("volume name empty")
	}
	if got[0].Driver == "" {
		t.Error("volume driver empty")
	}
}

func TestFormatPorts(t *testing.T) {
	if FormatPorts(nil) != "-" {
		t.Fatal("nil ports")
	}
	ports := []PortMapping{
		{HostPort: 80, ContainerPort: 8080},
		{HostPort: 443, ContainerPort: 8443},
	}
	want := "80→8080, 443→8443"
	if got := FormatPorts(ports); got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestIsRunning(t *testing.T) {
	c := Container{Status: "running"}
	if !c.IsRunning() {
		t.Fatal("running")
	}
	c.Status = "stopped"
	if c.IsRunning() {
		t.Fatal("stopped")
	}
}

func TestSplitRef(t *testing.T) {
	repo, tag := splitRef("docker.io/library/alpine:latest")
	if repo != "docker.io/library/alpine" || tag != "latest" {
		t.Fatalf("got %s %s", repo, tag)
	}
	repo, tag = splitRef("alpine")
	if repo != "alpine" || tag != "latest" {
		t.Fatalf("got %s %s", repo, tag)
	}
}

func TestCreatedParse(t *testing.T) {
	raw := []cliContainer{{
		ID: "x",
		Configuration: cliContainerConfig{
			ID:           "x",
			CreationDate: "2026-08-05T16:08:35Z",
			Image:        cliImageRef{Reference: "alpine:latest"},
		},
		Status: cliContainerStatus{State: "running"},
	}}
	got := mapContainers(raw)
	want := time.Date(2026, 8, 5, 16, 8, 35, 0, time.UTC)
	if !got[0].Created.Equal(want) {
		t.Fatalf("want %v got %v", want, got[0].Created)
	}
}
