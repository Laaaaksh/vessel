package backend

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// cliImage is the raw JSON shape from `container image list --json`.
type cliImage struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       int64  `json:"size"`
	Created    string `json:"created"`
}

// ListImages returns all local images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	var raw []cliImage
	if err := c.runJSON(ctx, &raw, "image", "list", "--json"); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return mapImages(raw), nil
}

// RemoveImage removes an image by ID or name.
func (c *Client) RemoveImage(ctx context.Context, id string) error {
	_, err := c.run(ctx, "image", "rm", id)
	return err
}

// PullImage pulls an image (blocking until complete).
func (c *Client) PullImage(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "pull", ref)
	return err
}

func mapImages(raw []cliImage) []Image {
	out := make([]Image, 0, len(raw))
	for _, r := range raw {
		img := Image{
			ID:         r.ID,
			Repository: r.Repository,
			Tag:        r.Tag,
			Size:       r.Size,
		}
		if t, err := time.Parse(time.RFC3339, r.Created); err == nil {
			img.Created = t
		}
		out = append(out, img)
	}
	return out
}

// FormatRef returns the full image reference "repo:tag".
func FormatRef(img Image) string {
	if img.Tag == "" || img.Tag == "<none>" {
		return img.Repository
	}
	return strings.Join([]string{img.Repository, img.Tag}, ":")
}
