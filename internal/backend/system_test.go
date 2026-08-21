package backend

import (
	"context"
	"testing"
	"time"
)

func TestMapSystemStatus_whenRunning(t *testing.T) {
	raw := loadFixture[cliSystemStatus](t, "system-status.json")
	got := mapSystemStatus(raw)
	if got.Status != systemStatusRunning {
		t.Errorf("status want %q, got %q", systemStatusRunning, got.Status)
	}
	if !got.IsRunning() {
		t.Error("IsRunning() want true")
	}
	if got.Version == "" {
		t.Error("version empty")
	}
	if got.AppRoot == "" {
		t.Error("appRoot empty")
	}
	if got.InstallRoot == "" {
		t.Error("installRoot empty")
	}
}

func TestMapSystemStatus_whenServicesNeverStarted(t *testing.T) {
	raw := loadFixture[cliSystemStatus](t, "system-status-down.json")
	got := mapSystemStatus(raw)
	if got.Status != "unregistered" {
		t.Errorf("status want unregistered, got %q", got.Status)
	}
	if got.IsRunning() {
		t.Error("IsRunning() want false")
	}
}

func TestMapDiskUsage_fromLiveFixture(t *testing.T) {
	raw := loadFixture[cliDiskUsage](t, "system-df.json")
	got := mapDiskUsage(raw)
	if got.Images.Total != 2 {
		t.Errorf("images total want 2, got %d", got.Images.Total)
	}
	if got.Images.Active != 1 {
		t.Errorf("images active want 1, got %d", got.Images.Active)
	}
	if got.Images.SizeBytes == 0 {
		t.Error("images sizeBytes empty")
	}
	if got.Containers.Total != 3 {
		t.Errorf("containers total want 3, got %d", got.Containers.Total)
	}
	if got.Volumes.ReclaimableBytes == 0 {
		t.Error("volumes reclaimableBytes empty")
	}
}

func TestClient_SystemStatus_fake_whenRunning(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.SystemStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsRunning() {
		t.Error("IsRunning() want true")
	}
}

// The CLI exits non-zero here even though it still prints a valid,
// parseable "unregistered" body - the down state this view exists to show,
// not a failure to report on - so this must return a result, not an error.
func TestClient_SystemStatus_fake_whenServicesDown(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	t.Setenv("FAKE_CONTAINER_SYSTEM_DOWN", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.SystemStatus(ctx)
	if err != nil {
		t.Fatalf("expected a parsed down status, not an error: %v", err)
	}
	if got.IsRunning() {
		t.Error("IsRunning() want false")
	}
}

func TestClient_DiskUsage_fake_whenRunning(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.DiskUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Images.Total == 0 {
		t.Error("images total empty")
	}
}

// Unlike system status, "system df" has no JSON body to recover on a
// down-services failure - the CLI prints a plain-text error - so this must
// surface as a real error.
func TestClient_DiskUsage_fake_whenServicesDown(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	t.Setenv("FAKE_CONTAINER_SYSTEM_DOWN", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.DiskUsage(ctx); err == nil {
		t.Fatal("expected an error")
	}
}
