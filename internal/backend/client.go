package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// defaultTimeout bounds a routine CLI invocation: lists, inspects,
	// lifecycle verbs, single-target deletes. It stays short so a hung
	// quick command still fails fast instead of wedging a poll or action.
	defaultTimeout = 10 * time.Second

	// globalTimeout bounds one invocation that is known to run long: an
	// image transfer (tag/save/load/push) or a whole-store prune sweep. It
	// matches the identically named outer bound internal/ui wraps these
	// verbs in, so the per-call budget the client applies here is the same
	// window the UI already promises; TestLongOperationBudgets pins the pair.
	globalTimeout = 120 * time.Second

	// confirmTimeout bounds one batched multi-target delete. The CLI call
	// carries every id at once, so its budget must cover N deletions rather
	// than one, and it likewise matches internal/ui's confirmTimeout for
	// the confirmed removal that issued it.
	confirmTimeout = 60 * time.Second

	// cliName is the Apple Mac containers binary.
	cliName = "container"
	cmdLogN = 40
)

// errNoDeleteTargets rejects an empty variadic delete. The CLI needs an explicit
// --all to wipe everything, so a no-argument call destroys nothing; it is a
// caller bug that would otherwise surface only as a confusing CLI usage error.
var errNoDeleteTargets = fmt.Errorf("delete requires at least one target")

// servicesDownHints are fragments the container CLI emits (plugin-gated verbs,
// e.g. image prune, volume create, volume prune) when the container system
// services have not been started yet. See docs/APPLE_CONTAINER_MATRIX.md.
var servicesDownHints = []string{
	"Plugins are unavailable",
	"system services are not running",
	"has been started with `container system start`",
	"has been started with \"container system start\"",
}

// IsServicesDown reports whether err is the "run `container system start` first"
// failure class: system services down, so plugin-backed verbs fail.
func IsServicesDown(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, h := range servicesDownHints {
		if strings.Contains(msg, h) {
			return true
		}
	}
	return false
}

// Client is the adapter that shells out to the container CLI.
type Client struct {
	binary  string
	timeout time.Duration

	logMu  sync.Mutex
	cmdLog []string
}

// NewClient creates a Client with the system container binary.
func NewClient() (*Client, error) {
	path, err := exec.LookPath(cliName)
	if err != nil {
		return nil, fmt.Errorf("container CLI not found: %w (is Apple Mac containers installed?)", err)
	}
	return &Client{binary: path, timeout: defaultTimeout}, nil
}

// NewClientWithBinary creates a Client with an explicit binary path (useful for tests).
func NewClientWithBinary(path string) *Client {
	return &Client{binary: path, timeout: defaultTimeout}
}

// CommandLog returns a copy of recent CLI invocations (newest last).
func (c *Client) CommandLog() []string {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	out := make([]string, len(c.cmdLog))
	copy(out, c.cmdLog)
	return out
}

func (c *Client) recordCmd(args []string) {
	line := "container " + strings.Join(args, " ")
	c.logMu.Lock()
	defer c.logMu.Unlock()
	c.cmdLog = append(c.cmdLog, line)
	if len(c.cmdLog) > cmdLogN {
		c.cmdLog = c.cmdLog[len(c.cmdLog)-cmdLogN:]
	}
}

// CLIError is a failed container CLI invocation. It keeps stderr apart from the
// command line so callers classify a failure by what the CLI reported rather
// than by the arguments it was handed.
type CLIError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CLIError) Error() string {
	return fmt.Sprintf("container %v: %v (stderr: %s)", e.Args, e.Err, e.Stderr)
}

func (e *CLIError) Unwrap() error { return e.Err }

// run executes a container CLI subcommand under the client's default budget
// and returns its stdout.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	out, err := c.runRaw(ctx, args...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// runWithTimeout is run with an explicit per-call budget that replaces the
// default cap instead of being clamped by it. Operations known to outlast
// defaultTimeout — image transfers, prune sweeps, batched deletes — pass one
// of the named budget constants; everything else keeps the short default so
// a hung quick command still dies fast.
func (c *Client) runWithTimeout(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	out, err := c.runRawWithTimeout(ctx, timeout, args...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// runRaw executes a container CLI subcommand like run, but also returns
// whatever stdout the process wrote when it exits non-zero, instead of
// discarding it. Most commands print nothing useful on failure, but "system
// status" prints a real, parseable body even on a non-zero exit; callers
// that need that body use this directly.
func (c *Client) runRaw(ctx context.Context, args ...string) ([]byte, error) {
	return c.runRawWithTimeout(ctx, c.timeout, args...)
}

// runRawWithTimeout is the shared execution core: every invocation funnels
// through here, and the per-call timeout it wraps is whatever the entry point
// above chose — the default for quick commands, an explicit named budget for
// the known-long set. A non-positive timeout falls back to the default rather
// than killing the process immediately.
func (c *Client) runRawWithTimeout(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	c.recordCmd(args)
	if timeout <= 0 {
		timeout = c.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &CLIError{Args: args, Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}

// runJSON executes a subcommand and JSON-decodes its output into dst.
func (c *Client) runJSON(ctx context.Context, dst any, args ...string) error {
	out, err := c.run(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, dst); err != nil {
		return fmt.Errorf("container %v: JSON decode: %w", args, err)
	}
	return nil
}
