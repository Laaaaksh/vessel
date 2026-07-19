package backend

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cliContainer is the raw JSON shape from `container list --format json`.
// Field names match Apple's CLI output; we re-map into our Container type.
type cliContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Created string `json:"created"` // RFC3339
	Ports   []struct {
		HostPort      int    `json:"hostPort"`
		ContainerPort int    `json:"containerPort"`
		Protocol      string `json:"protocol"`
	} `json:"ports"`
	Env    []string          `json:"env"`
	Labels map[string]string `json:"labels"`
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

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	_, err := c.run(ctx, "restart", id)
	return err
}

// RemoveContainer removes a container (must be stopped first).
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	_, err := c.run(ctx, "rm", id)
	return err
}

func mapContainers(raw []cliContainer) []Container {
	out := make([]Container, 0, len(raw))
	for _, r := range raw {
		c := Container{
			ID:     r.ID,
			Name:   strings.TrimPrefix(r.Name, "/"),
			Image:  r.Image,
			Status: r.Status,
			Env:    r.Env,
			Labels: r.Labels,
		}
		if t, err := time.Parse(time.RFC3339, r.Created); err == nil {
			c.Created = t
		}
		for _, p := range r.Ports {
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
		parts = append(parts, strconv.Itoa(p.HostPort)+"→"+strconv.Itoa(p.ContainerPort))
	}
	return strings.Join(parts, ", ")
}
