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
