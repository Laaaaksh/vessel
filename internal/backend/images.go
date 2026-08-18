package backend

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Apple container runs Linux guests regardless of the host OS, so the guest
// platform to match a manifest variant against is always linux/GOARCH.
const guestOS = "linux"

// Apple container image list JSON (container 1.2.x).
type cliImage struct {
	Configuration struct {
		CreationDate string `json:"creationDate"`
		Name         string `json:"name"`
		Descriptor   struct {
			Size int64 `json:"size"`
		} `json:"descriptor"`
	} `json:"configuration"`
	ID       string            `json:"id"`
	Variants []cliImageVariant `json:"variants"`
}

// cliImageVariant is one per-platform manifest beneath a multi-arch index.
type cliImageVariant struct {
	Platform struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"platform"`
	Size int64 `json:"size"`
}

// ListImages returns all local images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	var raw []cliImage
	if err := c.runJSON(ctx, &raw, "image", "list", "--format", "json"); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return mapImages(raw), nil
}

// RemoveImage removes one or more images by ID or name in a single call.
func (c *Client) RemoveImage(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return errNoDeleteTargets
	}
	_, err := c.run(ctx, append([]string{"image", "delete"}, ids...)...)
	return err
}

// PullImage pulls an image (blocking until complete).
func (c *Client) PullImage(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "image", "pull", ref)
	return err
}

// PruneImages removes unused images.
func (c *Client) PruneImages(ctx context.Context) error {
	_, err := c.run(ctx, "image", "prune")
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
			Size:       imageSize(r),
		}
		if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
			img.Created = t
		}
		out = append(out, img)
	}
	return out
}

// imageSize reports the size of the manifest this host would actually run.
// configuration.descriptor.size is the size of the index manifest itself — a
// few KiB — not of the image, so it is only a last resort.
func imageSize(r cliImage) int64 {
	var largest int64
	for _, v := range r.Variants {
		// "unknown/unknown" variants are attestation manifests, not images.
		if v.Platform.OS == "unknown" || v.Platform.Architecture == "unknown" {
			continue
		}
		if v.Platform.OS == guestOS && v.Platform.Architecture == runtime.GOARCH {
			return v.Size
		}
		if v.Size > largest {
			largest = v.Size
		}
	}
	if largest > 0 {
		return largest
	}
	return r.Configuration.Descriptor.Size
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
