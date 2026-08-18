package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func fakeBinary(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	bin := filepath.Join(filepath.Dir(file), "fakecli", "container")
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("fake cli missing: %v", err)
	}
	return bin
}

func TestClient_ListContainers_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items, err := c.ListContainers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected containers from fake list")
	}
}

func TestClient_ListImages_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items, err := c.ListImages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected images")
	}
}

func TestClient_ListVolumes_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items, err := c.ListVolumes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected volumes")
	}
}

func TestClient_Lifecycle_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.StartContainer(ctx, "vessel-probe"); err != nil {
		t.Fatal(err)
	}
	if err := c.StopContainer(ctx, "vessel-probe"); err != nil {
		t.Fatal(err)
	}
	if err := c.RestartContainer(ctx, "vessel-probe"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveContainer(ctx, "vessel-probe"); err != nil {
		t.Fatal(err)
	}
}

func TestClient_StreamLogs_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := make(chan LogLine, 8)
	if err := c.StreamLogs(ctx, "vessel-probe", ch); err != nil {
		t.Fatal(err)
	}
	// StreamLogs owns ch: ranging must terminate without a send-on-closed panic.
	n := 0
	for line := range ch {
		if line.ContainerID != "vessel-probe" {
			t.Fatalf("wrong container id: %q", line.ContainerID)
		}
		n++
	}
	if n == 0 {
		t.Fatal("expected streamed log lines")
	}
}

func TestClient_StreamLogs_badBinaryClosesChannel(t *testing.T) {
	c := NewClientWithBinary(filepath.Join(t.TempDir(), "does-not-exist"))
	ch := make(chan LogLine, 1)
	if err := c.StreamLogs(context.Background(), "x", ch); err == nil {
		t.Fatal("expected a start error")
	}
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed on start failure")
	}
}

func TestClient_InspectContainer_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.InspectContainer(ctx, "vessel-probe")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "vessel-probe" {
		t.Fatalf("want vessel-probe, got %q", got.Name)
	}
}

func TestClient_TailLogs_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lines, err := c.TailLogs(ctx, "vessel-probe", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Fatalf("want 5 lines, got %d", len(lines))
	}
}

func TestIsServicesDown(t *testing.T) {
	cases := []struct {
		name string
		err  error
		down bool
	}{
		{name: "nil", err: nil, down: false},
		{name: "plugins unavailable", err: fmt.Errorf("container [image prune]: exit status 1 (stderr: Error: Plugins are unavailable. Start the container system services and retry:\n\n    container system start\n)"), down: true},
		{name: "plugins unavailable short", err: errors.New("Plugins are unavailable"), down: true},
		{name: "system start hint mid-error", err: errors.New("other: has been started with `container system start`"), down: true},
		{name: "unrelated", err: errors.New("container [list]: exit status 1 (stderr: boom)"), down: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsServicesDown(tc.err); got != tc.down {
				t.Fatalf("IsServicesDown(%v) = %v, want %v", tc.err, got, tc.down)
			}
		})
	}
}
