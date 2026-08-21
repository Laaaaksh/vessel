package backend

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// errNetworkNotFound is returned when an inspect targets a network the CLI
// does not know about (an empty result array rather than a nonzero exit).
var errNetworkNotFound = errors.New("network not found")

// Apple container network list/inspect JSON (container 1.2.x).
type cliNetwork struct {
	Configuration struct {
		CreationDate string `json:"creationDate"`
		Mode         string `json:"mode"`
		Name         string `json:"name"`
	} `json:"configuration"`
	ID     string `json:"id"`
	Status struct {
		IPv4Gateway string `json:"ipv4Gateway"`
		IPv4Subnet  string `json:"ipv4Subnet"`
	} `json:"status"`
}

// ListNetworks returns all networks.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	var raw []cliNetwork
	if err := c.runJSON(ctx, &raw, "network", "list", "--format", "json"); err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}
	return mapNetworks(raw), nil
}

// NetworkInspect returns detailed info for a single network.
func (c *Client) NetworkInspect(ctx context.Context, name string) (*Network, error) {
	var raw []cliNetwork
	if err := c.runJSON(ctx, &raw, "network", "inspect", name); err != nil {
		return nil, fmt.Errorf("inspect network %s: %w", name, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: %s", errNetworkNotFound, name)
	}
	ns := mapNetworks(raw)
	return &ns[0], nil
}

func mapNetworks(raw []cliNetwork) []Network {
	out := make([]Network, 0, len(raw))
	for _, r := range raw {
		name := r.Configuration.Name
		if name == "" {
			name = r.ID
		}
		n := Network{
			Name:    name,
			Mode:    r.Configuration.Mode,
			Gateway: r.Status.IPv4Gateway,
			Subnet:  r.Status.IPv4Subnet,
		}
		if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
			n.Created = t
		}
		out = append(out, n)
	}
	return out
}
