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
	Config cliVariantConfig `json:"config"`
	// Digest is the manifest digest of this variant.
	Digest   string `json:"digest"`
	Platform struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Variant      string `json:"variant"`
	} `json:"platform"`
	Size int64 `json:"size"`
}

// cliVariantConfig is the OCI config blob of a single-image manifest.
type cliVariantConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant"`
	Created      string `json:"created"`
	Config       struct {
		Cmd        []string `json:"Cmd"`
		Env        []string `json:"Env"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

// ListImages returns all local images.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	var raw []cliImage
	if err := c.runJSON(ctx, &raw, "image", "list", "--format", "json"); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return mapImages(raw), nil
}

// ImageInspect returns the full inspection of a single image identified by its
// reference (name or name:tag). The CLI reports unknown numeric IDs as not
// found, so callers should pass the reference.
func (c *Client) ImageInspect(ctx context.Context, ref string) (*ImageInspect, error) {
	var raw []cliImage
	if err := c.runJSON(ctx, &raw, "image", "inspect", ref); err != nil {
		return nil, fmt.Errorf("inspect image %s: %w", ref, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("image not found: %s", ref)
	}
	return mapImageInspect(raw[0]), nil
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

// mapImageInspect maps one inspected image to the enriched domain type. The
// per-variant config that matters (digest, size, cmd, env, working directory,
// layer count) is taken from the variant this host would run, mirroring the
// running-platform choice in imageSize.
func mapImageInspect(r cliImage) *ImageInspect {
	repo, tag := splitRef(r.Configuration.Name)
	ins := &ImageInspect{
		ID:         r.ID,
		Repository: repo,
		Tag:        tag,
	}
	if t, err := time.Parse(time.RFC3339, r.Configuration.CreationDate); err == nil {
		ins.Created = t
	}
	for _, v := range r.Variants {
		p := v.Platform
		if p.OS == "unknown" || p.Architecture == "unknown" {
			continue
		}
		ins.Platforms = append(ins.Platforms, ImagePlatform{
			OS:           p.OS,
			Architecture: p.Architecture,
			Variant:      p.Variant,
			Digest:       v.Digest,
			Size:         v.Size,
		})
	}
	if v := runVariant(r); v != nil {
		ins.Digest = v.Digest
		ins.Size = v.Size
		if t, err := time.Parse(time.RFC3339, v.Config.Created); err == nil {
			ins.Created = t
		}
		ins.Cmd = v.Config.Config.Cmd
		ins.WorkingDir = v.Config.Config.WorkingDir
		ins.Env = v.Config.Config.Env
		ins.LayerCount = len(v.Config.RootFS.DiffIDs)
	}
	return ins
}

// runVariant returns the manifest variant this host would actually run: the
// one matching linux/GOARCH, else the largest remaining image variant, so a
// single-arch image still resolves to real data. Returns nil when the image
// carries no image manifest at all.
func runVariant(r cliImage) *cliImageVariant {
	var largest *cliImageVariant
	for i := range r.Variants {
		v := &r.Variants[i]
		// "unknown/unknown" variants are attestation manifests, not images.
		if v.Platform.OS == "unknown" || v.Platform.Architecture == "unknown" {
			continue
		}
		if v.Platform.OS == guestOS && v.Platform.Architecture == runtime.GOARCH {
			return v
		}
		if largest == nil || v.Size > largest.Size {
			largest = v
		}
	}
	return largest
}

// imageSize reports the size of the manifest this host would actually run.
// configuration.descriptor.size is the size of the index manifest itself — a
// few KiB — not of the image, so it is only a last resort.
func imageSize(r cliImage) int64 {
	if v := runVariant(r); v != nil && v.Size > 0 {
		return v.Size
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
