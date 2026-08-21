package backend

import (
	"context"
	"errors"
	"fmt"
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

// lastCmdOrEmpty is lastCmd's non-fatal twin: an empty log is a legitimate
// expectation here, because a guard that rejects its input must not shell out.
func lastCmdOrEmpty(c *Client) string {
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
	if got := lastCmdOrEmpty(c); got != "container image tag alpine:latest vessel/alpine:probe" {
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
	if got := lastCmdOrEmpty(c); got != "container image save --output /tmp/vessel-out.tar alpine:latest" {
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
	if got := lastCmdOrEmpty(c); got != "container image load --input "+path {
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
	if got := lastCmdOrEmpty(c); got != "" {
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
	if got := lastCmdOrEmpty(c); got != "container image push vessel/alpine:probe" {
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

func TestClient_PushImage_authHintStaysOnOneLine(t *testing.T) {
	push := func(mode string) error {
		t.Setenv("FAKE_CONTAINER_FAIL_PUSH", mode)
		c := NewClientWithBinary(fakeBinary(t))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return c.PushImage(ctx, "alpine:latest")
	}
	authErr, plainErr := push("auth"), push("generic")
	if authErr == nil || plainErr == nil {
		t.Fatal("expected both fake pushes to fail")
	}
	if !strings.Contains(authErr.Error(), "container registry login") {
		t.Fatalf("auth error must name the login command, got: %v", authErr)
	}
	got, want := strings.Count(authErr.Error(), "\n"), strings.Count(plainErr.Error(), "\n")
	if got != want {
		t.Fatalf("auth hint added %d line breaks to a footer that renders one row (auth=%d, plain=%d)", got-want, got, want)
	}
}

func TestPushTarget_namesOnlyAHostTheReferenceCarries(t *testing.T) {
	named := map[string]string{
		"ghcr.io/vessel/alpine:probe":     "ghcr.io",
		"registry.local:5000/team/app:v2": "registry.local:5000",
		"localhost:5000/team/app:v2":      "localhost:5000",
		"localhost/team/app:v2":           "localhost",
	}
	for ref, want := range named {
		got, ok := PushTarget(ref)
		if !ok || got != want {
			t.Errorf("PushTarget(%q) = %q,%v, want %q,true", ref, got, ok, want)
		}
	}
	// An unqualified reference resolves against whatever default registry the
	// CLI is configured with, which vessel does not read — so it must not guess.
	for _, ref := range []string{"alpine:latest", "vessel/alpine:probe"} {
		if got, ok := PushTarget(ref); ok || got != "" {
			t.Errorf("PushTarget(%q) = %q,%v, want a refusal to guess", ref, got, ok)
		}
	}
}

func TestExactRef(t *testing.T) {
	if ref, ok := ExactRef(Image{Repository: "alpine", Tag: "latest"}); !ok || ref != "alpine:latest" {
		t.Fatalf("tagged image: got %q %v", ref, ok)
	}
	for _, img := range []Image{
		{Repository: "alpine", Tag: ""},
		{Repository: "alpine", Tag: "<none>"},
		{Repository: "", Tag: "latest"},
	} {
		if ref, ok := ExactRef(img); ok {
			t.Errorf("ExactRef(%+v) resolved to %q, want a refusal", img, ref)
		}
	}
}

func TestClient_PushImage_authHintNotTriggeredByImageName(t *testing.T) {
	// The CLI echoes the reference into stderr, so a repository whose own words
	// read like a credentials failure must not be classified as one.
	for _, ref := range []string{
		"myorg/authentication-service:v1",
		"myorg/unauthorized-proxy:v1",
		"myorg/authentication-service:401-unauthorised",
	} {
		t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "generic")
		c := NewClientWithBinary(fakeBinary(t))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.PushImage(ctx, ref)
		cancel()
		if err == nil {
			t.Fatalf("%s: expected a failure from the fake", ref)
		}
		if !strings.Contains(err.Error(), ref) {
			t.Fatalf("precondition: the fake should echo %q into stderr, got: %v", ref, err)
		}
		if strings.Contains(err.Error(), "container registry login") {
			t.Errorf("a non-auth failure must not be diagnosed from the image name %q, got: %v", ref, err)
		}
	}
}

func TestClient_SaveImage_expandsHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.SaveImage(ctx, "alpine:latest", "~/out.tar"); err != nil {
		t.Fatal(err)
	}
	want := "container image save --output " + filepath.Join(home, "out.tar") + " alpine:latest"
	if got := lastCmdOrEmpty(c); got != want {
		t.Fatalf("save must expand ~: got %q want %q", got, want)
	}
}

func TestClient_LoadImage_expandsHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, "in.tar"), []byte("oci-archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.LoadImage(ctx, "~/in.tar"); err != nil {
		t.Fatalf("a home-relative archive that exists must load: %v", err)
	}
	want := "container image load --input " + filepath.Join(home, "in.tar")
	if got := lastCmdOrEmpty(c); got != want {
		t.Fatalf("load must expand ~: got %q want %q", got, want)
	}
}

// Both spellings of the dismissal the 403 path must never make. A guard on only
// one of them lets the claim return under the other, so every 403 surface is
// held to both.
const (
	loginDismissalContracted = "won't help"
	loginDismissalSpelledOut = "will not help"
)

func TestClient_PushImage_forbiddenIsNotACredentialsFailure(t *testing.T) {
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", "forbidden")
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := c.PushImage(ctx, "myorg/app:v1")
	if err == nil {
		t.Fatal("expected the forbidden push to fail")
	}
	msg := err.Error()
	if strings.Contains(msg, "rejected these credentials") {
		t.Errorf("a 403 does not establish that the credentials were rejected, got: %v", err)
	}
	if strings.Contains(msg, loginDismissalSpelledOut) {
		t.Errorf("a 403 must not claim that logging in again is useless, got: %v", err)
	}
	if strings.Contains(msg, loginDismissalContracted) {
		t.Errorf("a 403 must not claim that logging in again is useless, got: %v", err)
	}
	if !strings.Contains(msg, "lack write access") {
		t.Errorf("a 403 should mention the account may lack write access, got: %v", err)
	}
	if !strings.Contains(msg, "log in") {
		t.Errorf("a 403 should mention that logging in may be needed, got: %v", err)
	}
	if got := strings.Count(msg, "\n"); got != strings.Count(pushErrText(t, "generic"), "\n") {
		t.Errorf("the permission hint must stay on one footer row, got %d line breaks", got)
	}
}

func TestPushDenialNotice_distinguishesTheTwoRefusals(t *testing.T) {
	credentials := &CLIError{
		Stderr: "Error: ... 401 Unauthorized. Reason: Unknown, no credentials found for host registry-1.docker.io\n",
		Err:    errors.New("exit status 1"),
	}
	permission := &CLIError{
		Stderr: "Error: ... 403 Forbidden. Reason: requested access to the resource is denied\n",
		Err:    errors.New("exit status 1"),
	}
	if got := PushDenialNotice(credentials); got != PushAuthNotice {
		t.Errorf("401 notice = %q, want %q", got, PushAuthNotice)
	}
	if got := PushDenialNotice(permission); got != PushPermissionNotice {
		t.Errorf("403 notice = %q, want %q", got, PushPermissionNotice)
	}
	// The hint the footer renders is guarded elsewhere; the notice is the other
	// 403 surface, and it must not dismiss logging in either.
	notice := PushDenialNotice(permission)
	if strings.Contains(notice, loginDismissalSpelledOut) {
		t.Errorf("the 403 notice must not claim logging in is useless, got %q", notice)
	}
	if strings.Contains(notice, loginDismissalContracted) {
		t.Errorf("the 403 notice must not claim logging in is useless, got %q", notice)
	}
	if got := PushDenialNotice(&CLIError{Stderr: "Error: unexpected network failure\n", Err: errors.New("exit status 1")}); got != "" {
		t.Errorf("a non-refusal should offer no notice, got %q", got)
	}
	if got := PushDenialNotice(errors.New("plain")); got != "" {
		t.Errorf("a non-CLI error should offer no notice, got %q", got)
	}
}

func pushErrText(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("FAKE_CONTAINER_FAIL_PUSH", mode)
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := c.PushImage(ctx, "myorg/app:v1")
	if err == nil {
		t.Fatalf("expected the %s push to fail", mode)
	}
	return err.Error()
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
