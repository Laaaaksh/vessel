package backend

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Apple container list/inspect JSON (container 1.2.x).
type cliContainer struct {
	Configuration cliContainerConfig `json:"configuration"`
	ID            string             `json:"id"`
	Status        cliContainerStatus `json:"status"`
}

type cliContainerConfig struct {
	ID             string             `json:"id"`
	CreationDate   string             `json:"creationDate"`
	Image          cliImageRef        `json:"image"`
	InitProcess    cliInitProcess     `json:"initProcess"`
	Labels         map[string]string  `json:"labels"`
	PublishedPorts []cliPublishedPort `json:"publishedPorts"`
}

type cliImageRef struct {
	Reference string `json:"reference"`
}

type cliInitProcess struct {
	Environment []string `json:"environment"`
}

type cliPublishedPort struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"proto"`
}

type cliContainerStatus struct {
	State       string `json:"state"`
	StartedDate string `json:"startedDate"`
}

// ListContainers returns all containers (running and stopped).
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	var raw []cliContainer
	if err := c.runJSON(ctx, &raw, "list", "--all", "--format", "json"); err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	return mapContainers(raw), nil
}

// InspectContainer returns detailed info for a single container.
func (c *Client) InspectContainer(ctx context.Context, id string) (*Container, error) {
	var raw []cliContainer
	if err := c.runJSON(ctx, &raw, "inspect", id); err != nil {
		return nil, fmt.Errorf("inspect container %s: %w", id, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("container not found: %s", id)
	}
	cs := mapContainers(raw)
	return &cs[0], nil
}

// StartContainer starts a stopped container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	_, err := c.run(ctx, "start", id)
	return err
}

// StopContainer stops a running container.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	_, err := c.run(ctx, "stop", id)
	return err
}

// RestartContainer stops then starts a container.
// Apple's container CLI has no restart plugin as of 1.2.0.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	if err := c.StopContainer(ctx, id); err != nil {
		return err
	}
	return c.StartContainer(ctx, id)
}

// RemoveContainer deletes a container (force so running ones can be removed).
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	_, err := c.run(ctx, "delete", "--force", id)
	return err
}

// ShellCmd returns an *exec.Cmd ready for interactive shell attach.
// Callers should run it via tea.ExecProcess. The terminal is cleared before
// exec so bubbletea's last frame does not flash into the shell session.
func (c *Client) ShellCmd(id, shell string) *exec.Cmd {
	if shell == "" {
		shell = "/bin/sh"
	}
	// Clear screen then exec into the container. Using the host shell -c keeps
	// stdin/stdout a TTY for `container exec -it`.
	script := fmt.Sprintf(`printf '\033[2J\033[H'; exec %q exec -it %q %q`, c.binary, id, shell)
	return exec.Command("bash", "-lc", script)
}

// PruneContainers removes all stopped containers.
func (c *Client) PruneContainers(ctx context.Context) error {
	_, err := c.run(ctx, "prune")
	return err
}

// RunDetached starts a container from an image in the background.
func (c *Client) RunDetached(ctx context.Context, image string) error {
	_, err := c.run(ctx, "run", "-d", image)
	return err
}

func mapContainers(raw []cliContainer) []Container {
	out := make([]Container, 0, len(raw))
	for _, r := range raw {
		name := r.Configuration.ID
		if name == "" {
			name = r.ID
		}
		c := Container{
			ID:     r.ID,
			Name:   strings.TrimPrefix(name, "/"),
			Image:  r.Configuration.Image.Reference,
			Status: r.Status.State,
			Env:    r.Configuration.InitProcess.Environment,
			Labels: r.Configuration.Labels,
		}
		if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
			c.Created = t
		}
		for _, p := range r.Configuration.PublishedPorts {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			c.Ports = append(c.Ports, PortMapping{
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
				Protocol:      proto,
			})
		}
		out = append(out, c)
	}
	return out
}

// FormatPorts returns a compact ports string like "80→8080, 443→8443".
func FormatPorts(ports []PortMapping) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d→%d", p.HostPort, p.ContainerPort))
	}
	return strings.Join(parts, ", ")
}
