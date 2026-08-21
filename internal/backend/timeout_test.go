package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Budget-test knobs. The client's quick-command default is deliberately huge
// compared to a unit test, so these scenarios shrink it and stretch the fake
// instead of waiting out real seconds: shrunkDefault must be far below
// fakeSlowRuntime, which must in turn sit well inside every override budget
// the call sites pass.
const (
	shrunkDefault   = 50 * time.Millisecond
	fakeSlowRuntime = 300 * time.Millisecond
	overrideProbe   = 5 * time.Second
	scenarioBudget  = 5 * time.Second
)

// The budgets the known-long call sites pass are the client-side halves of a
// contract whose UI-side halves are internal/ui's identically sized outer
// bounds. Pinning both numbers here fails loudly if either side drifts
// without the pairing being re-decided.
const (
	wantQuickBudget  = 10 * time.Second
	wantGlobalBound  = 120 * time.Second
	wantConfirmBound = 60 * time.Second
)

// slowClient hands back a client wired to the fake CLI where every verb takes
// fakeSlowRuntime and the quick-command default is shrunk to shrunkDefault,
// so a scenario that succeeds ran under an override and one that fails was
// killed by the default.
func slowClient(t *testing.T) *Client {
	t.Helper()
	t.Setenv("FAKE_CONTAINER_SLEEP", fmt.Sprintf("%g", fakeSlowRuntime.Seconds()))
	c := NewClientWithBinary(fakeBinary(t))
	c.timeout = shrunkDefault
	return c
}

func scenarioCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), scenarioBudget)
}

func recordedCalls(c *Client) []string { return c.CommandLog() }

func TestLongOperationBudgets_matchInternalUIOuterBounds(t *testing.T) {
	if defaultTimeout != wantQuickBudget {
		t.Fatalf("quick-command default = %v, want %v", defaultTimeout, wantQuickBudget)
	}
	if globalTimeout != wantGlobalBound {
		t.Fatalf("transfer/sweep budget = %v, want %v (internal/ui globalTimeout)", globalTimeout, wantGlobalBound)
	}
	if confirmTimeout != wantConfirmBound {
		t.Fatalf("batched-delete budget = %v, want %v (internal/ui confirmTimeout)", confirmTimeout, wantConfirmBound)
	}
}

func TestNewClientWithBinary_appliesTheQuickDefaultBudget(t *testing.T) {
	if c := NewClientWithBinary(fakeBinary(t)); c.timeout != defaultTimeout {
		t.Fatalf("constructed client budget = %v, want %v", c.timeout, defaultTimeout)
	}
}

func TestRunWithTimeout_longerBudgetOverridesTheDefaultCap(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if _, err := c.runWithTimeout(ctx, overrideProbe, "prune"); err != nil {
		t.Fatalf("an explicit budget must outlast the shrunken default: %v", err)
	}
	want := []string{"container prune"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestRun_defaultCapStillKillsAHungQuickCommand(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if _, err := c.run(ctx, "start", "vessel-probe"); err == nil {
		t.Fatal("a hung quick command must still die under the shrunken default")
	}
	want := []string{"container start vessel-probe"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestRemoveImage_batchedIDs_runUnderBulkDeleteBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.RemoveImage(ctx, "alpine", "busybox", "debian"); err != nil {
		t.Fatalf("one batched call must hold one bulk-delete budget, not one-per-target: %v", err)
	}
	want := []string{"container image delete alpine busybox debian"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestRemoveVolume_batchedNames_runUnderBulkDeleteBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.RemoveVolume(ctx, "data", "logs"); err != nil {
		t.Fatalf("one batched call must hold one bulk-delete budget, not one-per-target: %v", err)
	}
	want := []string{"container volume delete data logs"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestRemoveContainer_singleTarget_keepsQuickDefaultBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.RemoveContainer(ctx, "vessel-probe"); err == nil {
		t.Fatal("a single-target delete is a quick command and must keep the default cap")
	}
	want := []string{"container delete --force vessel-probe"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestTagImage_runsUnderTransferBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.TagImage(ctx, "alpine:latest", "vessel/alpine:probe"); err != nil {
		t.Fatalf("tag shares the transfer verbs' budget: %v", err)
	}
	want := []string{"container image tag alpine:latest vessel/alpine:probe"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestSaveImage_runsUnderTransferBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	path := filepath.Join(t.TempDir(), "out.tar")
	if err := c.SaveImage(ctx, "alpine:latest", path); err != nil {
		t.Fatalf("a save slower than the quick default must survive under the transfer budget: %v", err)
	}
	want := []string{"container image save --output " + path + " alpine:latest"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestLoadImage_runsUnderTransferBudget(t *testing.T) {
	c := slowClient(t)
	path := filepath.Join(t.TempDir(), "in.tar")
	if err := os.WriteFile(path, []byte("oci-archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.LoadImage(ctx, path); err != nil {
		t.Fatalf("a load slower than the quick default must survive under the transfer budget: %v", err)
	}
	want := []string{"container image load --input " + path}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestPushImage_runsUnderTransferBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.PushImage(ctx, "vessel/alpine:probe"); err != nil {
		t.Fatalf("a push slower than the quick default must survive under the transfer budget: %v", err)
	}
	want := []string{"container image push vessel/alpine:probe"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestPruneImages_runsUnderSweepBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.PruneImages(ctx); err != nil {
		t.Fatalf("a whole-store sweep must hold the long budget: %v", err)
	}
	want := []string{"container image prune"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestPruneVolumes_runsUnderSweepBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.PruneVolumes(ctx); err != nil {
		t.Fatalf("a whole-store sweep must hold the long budget: %v", err)
	}
	want := []string{"container volume prune"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}

func TestPruneContainers_runsUnderSweepBudget(t *testing.T) {
	c := slowClient(t)
	ctx, cancel := scenarioCtx()
	defer cancel()

	if err := c.PruneContainers(ctx); err != nil {
		t.Fatalf("a whole-store sweep must hold the long budget: %v", err)
	}
	want := []string{"container prune"}
	if got := recordedCalls(c); len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("command log = %v, want exactly %v", got, want)
	}
}
