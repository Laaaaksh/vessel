package backend

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func lastCmd(c *Client) string {
	log := c.CommandLog()
	if len(log) == 0 {
		return ""
	}
	return log[len(log)-1]
}

func TestClient_TagImage_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.TagImage(ctx, "alpine:latest", "vessel/alpine:probe"); err != nil {
		t.Fatal(err)
	}
	if got := lastCmd(c); got != "container image tag alpine:latest vessel/alpine:probe" {
		t.Fatalf("tag argument order: got %q", got)
	}
}

func TestClient_SaveImage_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.SaveImage(ctx, "alpine:latest", "/tmp/vessel-out.tar"); err != nil {
		t.Fatal(err)
	}
	if got := lastCmd(c); got != "container image save --output /tmp/vessel-out.tar alpine:latest" {
		t.Fatalf("save argument order: got %q", got)
	}
}

func TestClient_LoadImage_fake(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vessel-in.tar")
	if err := os.WriteFile(path, []byte("oci-archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.LoadImage(ctx, path); err != nil {
		t.Fatal(err)
	}
	if got := lastCmd(c); got != "container image load --input "+path {
		t.Fatalf("load argument order: got %q", got)
	}
}

func TestClient_LoadImage_missingFile(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	missing := filepath.Join(t.TempDir(), "does-not-exist.tar")
	err := c.LoadImage(ctx, missing)
	if err == nil {
		t.Fatal("expected an error for a missing archive")
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("want a clear no-such-file error, got: %v", err)
	}
	if got := lastCmd(c); got != "" {
		t.Fatalf("missing path must not shell out, but recorded %q", got)
	}
}

func TestClient_PushImage_fake(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.PushImage(ctx, "vessel/alpine:probe"); err != nil {
		t.Fatal(err)
	}
	if got := lastCmd(c); got != "container image push vessel/alpine:probe" {
		t.Fatalf("push argument order: got %q", got)
	}
}

func TestClient_PushImage_authFailureNamesLogin(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "auth")
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := c.PushImage(ctx, "alpine:latest")
	if err == nil {
		t.Fatal("expected an auth failure from the fake")
	}
	if !strings.Contains(err.Error(), "container registry login") {
		t.Fatalf("auth error must name the login command, got: %v", err)
	}
}

func TestClient_PushImage_genericFailureNoHint(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "generic")
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := c.PushImage(ctx, "alpine:latest")
	if err == nil {
		t.Fatal("expected a failure from the fake")
	}
	if strings.Contains(err.Error(), "container registry login") {
		t.Fatalf("non-auth push error must not carry the login hint, got: %v", err)
	}
}
