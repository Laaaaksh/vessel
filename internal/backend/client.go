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
	defaultTimeout = 10 * time.Second
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

// run executes a container CLI subcommand and returns its stdout.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	out, err := c.runRaw(ctx, args...)
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
	c.recordCmd(args)
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("container %v: %w (stderr: %s)", args, err, stderr.String())
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
