package backend

import (
	"context"
	"fmt"
	"time"
)

// Apple container volume list/inspect JSON (container 1.2.x).
type cliVolume struct {
	Configuration struct {
		CreationDate string `json:"creationDate"`
		Driver       string `json:"driver"`
		Name         string `json:"name"`
		Source       string `json:"source"`
	} `json:"configuration"`
	ID string `json:"id"`
}

// ListVolumes returns all named volumes.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	var raw []cliVolume
	if err := c.runJSON(ctx, &raw, "volume", "list", "--format", "json"); err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	return mapVolumes(raw), nil
}

// RemoveVolume deletes a volume by name.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	_, err := c.run(ctx, "volume", "delete", name)
	return err
}

// CreateVolume creates a named volume.
func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.run(ctx, "volume", "create", name)
	return err
}

// PruneVolumes removes volumes with no container references.
func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.run(ctx, "volume", "prune")
	return err
}

func mapVolumes(raw []cliVolume) []Volume {
	out := make([]Volume, 0, len(raw))
	for _, r := range raw {
		name := r.Configuration.Name
		if name == "" {
			name = r.ID
		}
		v := Volume{
			Name:       name,
			Driver:     r.Configuration.Driver,
			Mountpoint: r.Configuration.Source,
		}
		if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
			v.Created = t
		}
		out = append(out, v)
	}
	return out
}
