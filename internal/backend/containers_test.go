package backend

import (
	"testing"
	"time"
)

func TestMapContainers_basic(t *testing.T) {
	raw := []cliContainer{
		{
			ID:      "abc123",
			Name:    "/my-app",
			Image:   "ubuntu:22.04",
			Status:  "running",
			Created: "2024-01-15T10:00:00Z",
			Ports: []struct {
				HostPort      int    `json:"hostPort"`
				ContainerPort int    `json:"containerPort"`
				Protocol      string `json:"protocol"`
			}{
				{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			},
			Env:    []string{"HOME=/root", "PATH=/usr/bin"},
			Labels: map[string]string{"app": "web"},
		},
	}

	got := mapContainers(raw)

	if len(got) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got))
	}
	c := got[0]

	if c.ID != "abc123" {
		t.Errorf("ID: want abc123, got %s", c.ID)
	}
	// Leading slash should be stripped from Name.
	if c.Name != "my-app" {
		t.Errorf("Name: want my-app, got %s", c.Name)
	}
	if c.Image != "ubuntu:22.04" {
		t.Errorf("Image: want ubuntu:22.04, got %s", c.Image)
	}
	if c.Status != "running" {
		t.Errorf("Status: want running, got %s", c.Status)
	}
	if c.Created.IsZero() {
		t.Error("Created should be parsed, got zero time")
	}
	want := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !c.Created.Equal(want) {
		t.Errorf("Created: want %v, got %v", want, c.Created)
	}
	if len(c.Ports) != 1 {
		t.Fatalf("expected 1 port mapping, got %d", len(c.Ports))
	}
	if c.Ports[0].HostPort != 8080 || c.Ports[0].ContainerPort != 80 {
		t.Errorf("Port mapping wrong: %+v", c.Ports[0])
	}
	if c.Ports[0].Protocol != "tcp" {
		t.Errorf("Protocol: want tcp, got %s", c.Ports[0].Protocol)
	}
}

func TestMapContainers_defaultProtocol(t *testing.T) {
	raw := []cliContainer{
		{
			ID:    "def456",
			Name:  "no-proto",
			Image: "alpine",
			Ports: []struct {
				HostPort      int    `json:"hostPort"`
				ContainerPort int    `json:"containerPort"`
				Protocol      string `json:"protocol"`
			}{
				{HostPort: 443, ContainerPort: 443, Protocol: ""},
			},
		},
	}

	got := mapContainers(raw)
	if got[0].Ports[0].Protocol != "tcp" {
		t.Errorf("empty protocol should default to tcp, got %s", got[0].Ports[0].Protocol)
	}
}

func TestMapContainers_badCreated(t *testing.T) {
	raw := []cliContainer{
		{
			ID:      "xyz",
			Name:    "bad-date",
			Image:   "scratch",
			Created: "not-a-date",
		},
	}

	got := mapContainers(raw)
	if !got[0].Created.IsZero() {
		t.Error("bad date should leave Created as zero time")
	}
}

func TestMapContainers_empty(t *testing.T) {
	got := mapContainers(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestFormatPorts_none(t *testing.T) {
	got := FormatPorts(nil)
	if got != "-" {
		t.Errorf("want -, got %s", got)
	}
}

func TestFormatPorts_single(t *testing.T) {
	ports := []PortMapping{{HostPort: 80, ContainerPort: 8080}}
	got := FormatPorts(ports)
	if got != "80→8080" {
		t.Errorf("want 80→8080, got %s", got)
	}
}

func TestFormatPorts_multiple(t *testing.T) {
	ports := []PortMapping{
		{HostPort: 80, ContainerPort: 8080},
		{HostPort: 443, ContainerPort: 8443},
	}
	got := FormatPorts(ports)
	want := "80→8080, 443→8443"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestIsRunning(t *testing.T) {
	tests := []struct {
		status  string
		running bool
	}{
		{"running", true},
		{"exited", false},
		{"created", false},
		{"", false},
	}
	for _, tt := range tests {
		c := Container{Status: tt.status}
		if got := c.IsRunning(); got != tt.running {
			t.Errorf("status %q: IsRunning()=%v, want %v", tt.status, got, tt.running)
		}
	}
}
