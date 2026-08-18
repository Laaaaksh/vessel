package backend

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestMapImageInspect_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliImage](t, "image-inspect.json")
	if len(raw) == 0 {
		t.Fatal("expected an image in image-inspect.json")
	}
	got := mapImageInspect(raw[0])
	if got.Repository == "" || got.Tag == "" {
		t.Errorf("repository/tag empty: %q %q", got.Repository, got.Tag)
	}
	if got.Created.IsZero() {
		t.Error("created empty")
	}
	if len(got.Platforms) == 0 {
		t.Fatal("expected platform variants")
	}
	// Attestation manifests are unknown/unknown and must be skipped.
	for _, p := range got.Platforms {
		if p.OS == "unknown" || p.Architecture == "unknown" {
			t.Errorf("attestation variant leaked into platforms: %+v", p)
		}
	}
	// The host-running variant must carry digest, size, and run config.
	if got.Digest == "" {
		t.Error("digest empty for host variant")
	}
	if got.Size == 0 {
		t.Error("size empty for host variant")
	}
	if len(got.Cmd) == 0 {
		t.Error("cmd empty for host variant")
	}
	if len(got.Env) == 0 {
		t.Error("env empty for host variant")
	}
	if got.LayerCount == 0 {
		t.Error("layer count zero for host variant")
	}
}

func TestMapImageInspect_matchesImageSizePlatform(t *testing.T) {
	raw := loadFixture[[]cliImage](t, "image-inspect.json")
	ins := mapImageInspect(raw[0])
	wantSize := imageSize(raw[0])
	if got := ins.Size; got != wantSize {
		t.Errorf("inspect size %d != list size %d", got, wantSize)
	}
	if runtime.GOARCH != "arm64" {
		t.Skip("fixture host variant is linux/arm64")
	}
	if ins.WorkingDir != "/" {
		t.Errorf("working dir want /, got %q", ins.WorkingDir)
	}
}

func TestClient_ImageInspect_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := c.ImageInspect(ctx, "docker.io/library/alpine:latest")
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository == "" {
		t.Error("repository empty")
	}
	if len(got.Platforms) == 0 {
		t.Error("expected platforms from fake inspect")
	}
}

func TestMapImageInspect_emptyVariants(t *testing.T) {
	var raw cliImage
	raw.Configuration.Name = "alpine:latest"
	raw.ID = "abc"
	got := mapImageInspect(raw)
	if got.Repository != "alpine" || got.Tag != "latest" {
		t.Errorf("got %q %q", got.Repository, got.Tag)
	}
	if got.Digest != "" || got.Size != 0 {
		t.Errorf("expected zero digest/size for empty variants, got %q %d", got.Digest, got.Size)
	}
	if len(got.Platforms) != 0 {
		t.Errorf("expected no platforms, got %d", len(got.Platforms))
	}
}
