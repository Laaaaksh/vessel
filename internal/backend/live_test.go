//go:build live

package backend_test

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
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

// TestLive_ImageSaveLoadRoundTrip verifies Phase 3's archive verbs against the
// installed container CLI rather than the fake: save writes a real OCI archive
// and load reads that same archive back. It stays non-destructive - it only
// re-tags a reference onto the digest that already carries it, and never
// deletes an image - so it is safe to run against a developer's live daemon.
//
// Run with: go test -tags=live ./internal/backend -run Live_Image -v
func TestLive_ImageSaveLoadRoundTrip(t *testing.T) {
	c, err := backend.NewClient()
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	imgs, err := c.ListImages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The 10s per-invocation cap makes a big image unusable here, so pick the
	// smallest named reference present; alpine-sized images save well inside it.
	var ref string
	var size int64
	for _, img := range imgs {
		r, ok := backend.ExactRef(img)
		if !ok {
			continue
		}
		if ref == "" || img.Size < size {
			ref, size = r, img.Size
		}
	}
	if ref == "" {
		t.Skip("no tagged local image to round-trip")
	}
	t.Logf("round-tripping %s (%d bytes)", ref, size)

	path := filepath.Join(t.TempDir(), "vessel-live.tar")
	if err := c.SaveImage(ctx, ref, path); err != nil {
		t.Fatalf("live image save is unavailable on this build: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("save reported success but wrote no archive: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("save wrote an empty archive")
	}
	entries := tarEntries(t, path)
	for _, want := range []string{"oci-layout", "index.json"} {
		if !entries[want] {
			t.Fatalf("archive is not an OCI layout: %v missing from %v", want, keysOf(entries))
		}
	}
	t.Logf("archive: %d bytes, %d entries, OCI layout confirmed", info.Size(), len(entries))

	if err := c.LoadImage(ctx, path); err != nil {
		t.Fatalf("live image load is unavailable on this build: %v", err)
	}
	after, err := c.ListImages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, img := range after {
		if r, ok := backend.ExactRef(img); ok && r == ref {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s is absent after a load of its own archive", ref)
	}
	t.Logf("load ok; %s still present among %d images", ref, len(after))

	// Re-tagging the reference onto the digest it already names is a no-op for
	// the daemon and still proves the argument order the CLI expects.
	if err := c.TagImage(ctx, ref, ref); err != nil {
		t.Fatalf("live image tag is unavailable on this build: %v", err)
	}

	missing := filepath.Join(t.TempDir(), "not-here.tar")
	err = c.LoadImage(ctx, missing)
	if err == nil {
		t.Fatal("loading a missing archive should fail")
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("want a plain no-such-file error, got: %v", err)
	}
	t.Logf("missing archive reported as: %v", err)
	t.Logf("CLI invocations: %v", c.CommandLog())
}

func tarEntries(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string]bool{}
	r := tar.NewReader(f)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("archive is not a readable tar: %v", err)
		}
		out[strings.TrimPrefix(h.Name, "./")] = true
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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
