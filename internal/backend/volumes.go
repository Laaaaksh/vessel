package backend

import (
	"context"
	"fmt"
	"time"
)

// Apple container volume list/inspect JSON (container 1.2.x).
type cliVolume struct {
	Configuration struct {
		CreationDate string            `json:"creationDate"`
		Driver       string            `json:"driver"`
		Format       string            `json:"format"`
		Labels       map[string]string `json:"labels"`
		Name         string            `json:"name"`
		Options      map[string]string `json:"options"`
		SizeInBytes  uint64            `json:"sizeInBytes"`
		Source       string            `json:"source"`
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

// VolumeInspect returns the full inspection of a single volume by name. The
// command prints JSON by default; --format json is not an accepted flag.
func (c *Client) VolumeInspect(ctx context.Context, name string) (*VolumeInspect, error) {
	var raw []cliVolume
	if err := c.runJSON(ctx, &raw, "volume", "inspect", name); err != nil {
		return nil, fmt.Errorf("inspect volume %s: %w", name, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("volume not found: %s", name)
	}
	return mapVolumeInspect(raw[0]), nil
}

// RemoveVolume deletes one or more volumes by name in a single call.
// The batched invocation shares one budget across every target, so it gets
// the confirmed-removal window rather than the quick default.
func (c *Client) RemoveVolume(ctx context.Context, names ...string) error {
	if len(names) == 0 {
		return errNoDeleteTargets
	}
	_, err := c.runWithTimeout(ctx, confirmTimeout, append([]string{"volume", "delete"}, names...)...)
	return err
}

// CreateVolume creates a named volume.
func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.run(ctx, "volume", "create", name)
	return err
}

// PruneVolumes removes volumes with no container references. A prune sweeps
// the whole store, so it runs under the long sweep budget rather than the
// quick default.
func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.runWithTimeout(ctx, globalTimeout, "volume", "prune")
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
			SizeBytes:  r.Configuration.SizeInBytes,
			Format:     r.Configuration.Format,
			Labels:     r.Configuration.Labels,
			Options:    r.Configuration.Options,
		}
		if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
			v.Created = t
		}
		out = append(out, v)
	}
	return out
}

// mapVolumeInspect maps one inspected volume to the enriched domain type.
func mapVolumeInspect(r cliVolume) *VolumeInspect {
	v := mapVolumes([]cliVolume{r})[0]
	return &VolumeInspect{
		Name:       v.Name,
		Driver:     v.Driver,
		Mountpoint: v.Mountpoint,
		Created:    v.Created,
		SizeBytes:  v.SizeBytes,
		Format:     v.Format,
		Labels:     v.Labels,
		Options:    v.Options,
	}
}
