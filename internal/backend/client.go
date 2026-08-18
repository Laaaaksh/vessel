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

// errNoDeleteTargets guards against a destructively broad no-argument delete:
// the CLI treats a bare "image delete" as deleting everything, so never issue it.
var errNoDeleteTargets = fmt.Errorf("delete requires at least one target")

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
	c.recordCmd(args)
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("container %v: %w (stderr: %s)", args, err, stderr.String())
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
