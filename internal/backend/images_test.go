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

func TestMapImageInspect_fallsBackToLargestVariantOffHostArch(t *testing.T) {
	var raw cliImage
	raw.Configuration.Name = "example.com/app:1.0"
	raw.Configuration.Descriptor.Size = 1234
	raw.ID = "sha256:abc"
	small := cliImageVariant{Digest: "sha256:small", Size: 10}
	small.Platform.OS = guestOS
	small.Platform.Architecture = "otherarch"
	big := cliImageVariant{Digest: "sha256:big", Size: 4096}
	big.Platform.OS = guestOS
	big.Platform.Architecture = "biggerarch"
	big.Config.Config.Cmd = []string{"/bin/app"}
	big.Config.Config.WorkingDir = "/srv"
	big.Config.Config.Env = []string{"PATH=/usr/bin"}
	big.Config.RootFS.DiffIDs = []string{"sha256:l1", "sha256:l2"}
	att := cliImageVariant{Digest: "sha256:att", Size: 99999}
	att.Platform.OS = "unknown"
	att.Platform.Architecture = "unknown"
	raw.Variants = []cliImageVariant{small, big, att}

	got := mapImageInspect(raw)
	if got.Digest != "sha256:big" {
		t.Errorf("digest = %q, want the largest image variant", got.Digest)
	}
	if got.Size != 4096 {
		t.Errorf("size = %d, want 4096", got.Size)
	}
	if want := imageSize(raw); got.Size != want {
		t.Errorf("inspect size %d != list size %d", got.Size, want)
	}
	if len(got.Cmd) != 1 || got.Cmd[0] != "/bin/app" {
		t.Errorf("cmd = %v, want [/bin/app]", got.Cmd)
	}
	if got.WorkingDir != "/srv" {
		t.Errorf("working dir = %q, want /srv", got.WorkingDir)
	}
	if len(got.Env) != 1 {
		t.Errorf("env = %v, want one entry", got.Env)
	}
	if got.LayerCount != 2 {
		t.Errorf("layer count = %d, want 2", got.LayerCount)
	}
	if len(got.Platforms) != 2 {
		t.Errorf("platforms = %d, want 2 (attestation excluded)", len(got.Platforms))
	}
}

func TestImageSize_descriptorFallbackWhenVariantsSizeless(t *testing.T) {
	var raw cliImage
	raw.Configuration.Descriptor.Size = 777
	v := cliImageVariant{Digest: "sha256:z"}
	v.Platform.OS = guestOS
	v.Platform.Architecture = runtime.GOARCH
	raw.Variants = []cliImageVariant{v}
	if got := imageSize(raw); got != 777 {
		t.Errorf("imageSize = %d, want descriptor fallback 777", got)
	}
}

func TestClient_ImageInspectIDMatchesListID(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := c.ListImages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one image")
	}
	ins, err := c.ImageInspect(ctx, FormatRef(list[0]))
	if err != nil {
		t.Fatal(err)
	}
	if ins.ID != list[0].ID {
		t.Errorf("inspect ID %q != list ID %q; the UI keys its inspect cache on this identity", ins.ID, list[0].ID)
	}
}
