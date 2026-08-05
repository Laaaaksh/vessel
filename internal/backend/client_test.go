package backend

import (
	"context"
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
