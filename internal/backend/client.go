package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	// cliName is the Apple Mac containers binary.
	cliName = "container"
)

// Client is the adapter that shells out to the container CLI.
type Client struct {
	binary  string
	timeout time.Duration
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

// run executes a container CLI subcommand and returns its stdout.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
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
