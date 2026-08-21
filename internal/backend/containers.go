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
	Mounts         []cliMount         `json:"mounts"`
	Networks       []cliNetConfig     `json:"networks"`
	Platform       cliPlatform        `json:"platform"`
	Resources      cliResources       `json:"resources"`
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

type cliMount struct {
	Destination string `json:"destination"`
	Source      string `json:"source"`
	Type        struct {
		Volume struct {
			Name string `json:"name"`
		} `json:"volume"`
	} `json:"type"`
}

type cliNetConfig struct {
	Network string `json:"network"`
	Options struct {
		Hostname string `json:"hostname"`
	} `json:"options"`
}

type cliNetStatus struct {
	Hostname    string `json:"hostname"`
	IPv4Address string `json:"ipv4Address"`
	Network     string `json:"network"`
}

type cliPlatform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type cliResources struct {
	CPUs          int    `json:"cpus"`
	MemoryInBytes uint64 `json:"memoryInBytes"`
}

type cliContainerStatus struct {
	State       string         `json:"state"`
	StartedDate string         `json:"startedDate"`
	Networks    []cliNetStatus `json:"networks"`
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

// StartContainer starts a stopped container. A state transition can outlast
// a quick list, so it holds the lifecycle budget rather than the default.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	_, err := c.runWithTimeout(ctx, lifecycleTimeout, "start", id)
	return err
}

// StopContainer stops a running container under the same lifecycle budget.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	_, err := c.runWithTimeout(ctx, lifecycleTimeout, "stop", id)
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
	shell = resolveShell(shell)
	// Clear screen then exec into the container. Using the host shell -c keeps
	// stdin/stdout a TTY for `container exec -it`.
	script := fmt.Sprintf(`printf '\033[2J\033[H'; exec %q exec -it %q %q`, c.binary, id, shell)
	return exec.Command("bash", "-lc", script)
}

// PruneContainers removes all stopped containers. A prune sweeps the whole
// store, so it runs under the long sweep budget rather than the quick default.
func (c *Client) PruneContainers(ctx context.Context) error {
	_, err := c.runWithTimeout(ctx, globalTimeout, "prune")
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
			CPUs:   r.Configuration.Resources.CPUs,
		}
		c.MemoryBytes = r.Configuration.Resources.MemoryInBytes
		if p := r.Configuration.Platform; p.OS != "" && p.Architecture != "" {
			c.Platform = p.OS + "/" + p.Architecture
		}
		for _, n := range r.Configuration.Networks {
			if c.Hostname == "" && n.Options.Hostname != "" {
				c.Hostname = n.Options.Hostname
			}
		}
		for _, n := range r.Status.Networks {
			net := Network{Name: n.Network, IP: n.IPv4Address}
			c.Networks = append(c.Networks, net)
			if c.Hostname == "" {
				c.Hostname = n.Hostname
			}
		}
		if len(c.Networks) == 0 {
			// A stopped container reports no network status, but the network
			// it is configured on is still known and worth showing; only the
			// runtime address is genuinely absent.
			for _, n := range r.Configuration.Networks {
				if n.Network != "" {
					c.Networks = append(c.Networks, Network{Name: n.Network})
				}
			}
		}
		for _, mt := range r.Configuration.Mounts {
			// A named volume reports source as the backing disk image on the
			// host (~95 chars of Application Support path), so the volume name
			// is the only human-meaningful identity. Bind mounts carry no
			// volume name and their source is the host path the user asked for.
			src := mt.Type.Volume.Name
			if src == "" {
				src = mt.Source
			}
			c.Mounts = append(c.Mounts, Mount{Source: src, Destination: mt.Destination})
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

// FormatNetworks returns a compact networks string like "default (192.168.64.2)".
func FormatNetworks(nets []Network) string {
	if len(nets) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(nets))
	for _, n := range nets {
		s := n.Name
		if n.IP != "" {
			s = n.Name + " (" + n.IP + ")"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
