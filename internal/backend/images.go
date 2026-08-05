package backend

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Apple container image list JSON (container 1.2.x).
type cliImage struct {
	Configuration struct {
		CreationDate string `json:"creationDate"`
		Name         string `json:"name"`
		Descriptor   struct {
			Size int64 `json:"size"`
		} `json:"descriptor"`
	} `json:"configuration"`
	ID string `json:"id"`
}

// ListImages returns all local images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	var raw []cliImage
	if err := c.runJSON(ctx, &raw, "image", "list", "--format", "json"); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return mapImages(raw), nil
}

// RemoveImage removes an image by ID or name.
func (c *Client) RemoveImage(ctx context.Context, id string) error {
	_, err := c.run(ctx, "image", "delete", id)
	return err
}

// PullImage pulls an image (blocking until complete).
func (c *Client) PullImage(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "image", "pull", ref)
	return err
}

func mapImages(raw []cliImage) []Image {
	out := make([]Image, 0, len(raw))
	for _, r := range raw {
		repo, tag := splitRef(r.Configuration.Name)
		img := Image{
			ID:         r.ID,
			Repository: repo,
			Tag:        tag,
			Size:       r.Configuration.Descriptor.Size,
		}
		if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
			img.Created = t
		}
		out = append(out, img)
	}
	return out
}

func splitRef(name string) (repo, tag string) {
	if name == "" {
		return "", ""
	}
	// A digest-pinned ref has no tag; the digest is not one.
	if i := strings.Index(name, "@"); i >= 0 {
		return name[:i], ""
	}
	// Take last ':' that looks like a tag (not a port in host:port/...)
	i := strings.LastIndex(name, ":")
	if i < 0 {
		return name, "latest"
	}
	if strings.Contains(name[i+1:], "/") {
		return name, "latest"
	}
	return name[:i], name[i+1:]
}

// FormatRef returns the full image reference "repo:tag".
func FormatRef(img Image) string {
	if img.Tag == "" || img.Tag == "<none>" {
		return img.Repository
	}
	return img.Repository + ":" + img.Tag
}
