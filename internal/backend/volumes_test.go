package backend

import (
	"context"
	"testing"
	"time"
)

func TestMapVolumes_includesInspectFields(t *testing.T) {
	raw := loadFixture[[]cliVolume](t, "volumes.json")
	got := mapVolumes(raw)
	if len(got) == 0 {
		t.Fatal("expected volumes")
	}
	v := got[0]
	if v.SizeBytes == 0 {
		t.Error("sizeBytes empty")
	}
	if v.Format == "" {
		t.Error("format empty")
	}
	if v.Labels == nil {
		t.Error("labels nil")
	}
	if v.Options == nil {
		t.Error("options nil")
	}
}

func TestMapVolumeInspect_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliVolume](t, "volume-inspect.json")
	if len(raw) == 0 {
		t.Fatal("expected a volume in volume-inspect.json")
	}
	got := mapVolumeInspect(raw[0])
	if got.Name == "" {
		t.Error("name empty")
	}
	if got.Driver != "local" {
		t.Errorf("driver want local, got %q", got.Driver)
	}
	if got.Format != "ext4" {
		t.Errorf("format want ext4, got %q", got.Format)
	}
	if got.SizeBytes != 549755813888 {
		t.Errorf("size want 549755813888, got %d", got.SizeBytes)
	}
	if got.Mountpoint == "" {
		t.Error("mountpoint empty")
	}
	if got.Labels == nil || got.Options == nil {
		t.Error("labels/options nil")
	}
}

func TestClient_VolumeInspect_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.VolumeInspect(ctx, "vessel-test-vol")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name == "" {
		t.Error("name empty")
	}
	if got.SizeBytes == 0 {
		t.Error("sizeBytes empty")
	}
}
