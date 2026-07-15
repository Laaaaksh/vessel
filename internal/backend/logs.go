package backend

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// LogLine is a single line from a container's log stream.
type LogLine struct {
	ContainerID string
	Text        string
}

// StreamLogs streams log lines from a container into ch until ctx is cancelled.
// It is the caller's responsibility to drain ch after cancellation.
func (c *Client) StreamLogs(ctx context.Context, id string, ch chan<- LogLine) error {
	cmd := exec.CommandContext(ctx, c.binary, "logs", "--follow", id)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		defer cmd.Wait() //nolint:errcheck // streaming termination is expected on cancel
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- LogLine{ContainerID: id, Text: sc.Text()}:
			}
		}
	}()

	return nil
}

// TailLogs returns the last n lines of a container's logs synchronously.
func (c *Client) TailLogs(ctx context.Context, id string, n int) ([]string, error) {
	out, err := c.run(ctx, "logs", "--tail", fmt.Sprint(n), id)
	if err != nil {
		return nil, err
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, nil
}
