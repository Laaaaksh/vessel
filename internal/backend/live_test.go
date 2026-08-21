//go:build live

package backend_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Laaaaksh/vessel/internal/backend"
)

// Run with: go test -tags=live ./internal/backend -run Live -v
func TestLive_ListAndLifecycle(t *testing.T) {
	c, err := backend.NewClient()
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cs, err := c.ListContainers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("containers: %d", len(cs))
	if len(cs) == 0 {
		t.Skip("no containers running for live test")
	}

	imgs, err := c.ListImages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("images: %d", len(imgs))

	vols, err := c.ListVolumes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("volumes: %d", len(vols))

	// The lifecycle half of this test stops and restarts a real container, so it
	// must only ever touch fixtures this project created. Never fall back to an
	// arbitrary container: scripts/smoke.sh runs this unattended.
	id := ""
	for _, x := range cs {
		if x.Name == "vessel-ports" {
			id = x.ID
			break
		}
		if id == "" && strings.HasPrefix(x.Name, "vessel-") {
			id = x.ID
		}
	}
	if id == "" {
		t.Skip("no vessel-* container present; skipping lifecycle test")
	}

	if err := c.StopContainer(ctx, id); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := c.StartContainer(ctx, id); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := c.RestartContainer(ctx, id); err != nil {
		t.Fatalf("restart: %v", err)
	}
	lines, err := c.TailLogs(ctx, id, 5)
	if err != nil {
		t.Logf("tail logs: %v (ok if empty)", err)
	} else {
		t.Logf("log lines: %d", len(lines))
	}
}

// TestLive_InspectDepth verifies image/volume inspect against the real CLI and
// that container list surfaces mounts/networks/resources. It requires at least
// one image, one volume and one running container.
func TestLive_InspectDepth(t *testing.T) {
	c, err := backend.NewClient()
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	imgs, err := c.ListImages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) > 0 {
		ins, err := c.ImageInspect(ctx, backend.FormatRef(imgs[0]))
		if err != nil {
			t.Fatalf("image inspect %s: %v", backend.FormatRef(imgs[0]), err)
		}
		if len(ins.Platforms) == 0 {
			t.Error("image inspect: expected platforms")
		}
		t.Logf("image %s: %d platforms, digest %s", backend.FormatRef(imgs[0]), len(ins.Platforms), ins.Digest)
	} else {
		t.Log("skip image inspect: no local images")
	}

	vols, err := c.ListVolumes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) > 0 {
		ins, err := c.VolumeInspect(ctx, vols[0].Name)
		if err != nil {
			t.Fatalf("volume inspect %s: %v", vols[0].Name, err)
		}
		if ins.Format == "" || ins.SizeBytes == 0 {
			t.Errorf("volume inspect %s: missing format/size (%q, %d)", vols[0].Name, ins.Format, ins.SizeBytes)
		}
		t.Logf("volume %s: format %s, size %d", ins.Name, ins.Format, ins.SizeBytes)
	} else {
		t.Log("skip volume inspect: no local volumes")
	}

	cs, err := c.ListContainers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundRich := false
	for _, cc := range cs {
		if cc.Platform != "" && (cc.CPUs > 0 || cc.MemoryBytes > 0) {
			foundRich = true
			t.Logf("container %s: platform %s cpus %d mem %d nets %d mounts %d",
				cc.Name, cc.Platform, cc.CPUs, cc.MemoryBytes, len(cc.Networks), len(cc.Mounts))
			break
		}
	}
	if !foundRich {
		t.Error("no running container exposed platform/resources over list")
	}
}
