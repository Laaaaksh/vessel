package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// TagImage creates a new reference for an existing image.
func (c *Client) TagImage(ctx context.Context, ref, newRef string) error {
	_, err := c.run(ctx, "image", "tag", ref, newRef)
	return err
}

// SaveImage writes one image to an OCI-compatible tar archive at path.
func (c *Client) SaveImage(ctx context.Context, ref, path string) error {
	path, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("save image: %w", err)
	}
	_, err = c.run(ctx, "image", "save", "--output", path, ref)
	return err
}

// LoadImage imports images from an OCI-compatible tar archive at path.
// The path is checked up front so a missing file reports plainly instead of
// surfacing as a raw CLI error.
func (c *Client) LoadImage(ctx context.Context, path string) error {
	path, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	_, err = c.run(ctx, "image", "load", "--input", path)
	return err
}

// ExpandPath resolves a leading "~" against the home directory. Prompts feed
// their text straight to exec, which performs no shell expansion, so without
// this a home-relative path that exists reports as missing.
func ExpandPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// FileExists reports whether path already names a regular file that writing to
// it would truncate.
func FileExists(path string) bool {
	p, err := ExpandPath(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// PushImage pushes an image to its registry.
func (c *Client) PushImage(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "image", "push", ref)
	if err != nil && isPushAuthError(err) {
		return fmt.Errorf("%w%s", err, pushAuthHint)
	}
	return err
}

// pushAuthHint tells the user how to authenticate when a push is rejected for
// credentials. Container registry login is intentionally out of vessel's scope:
// the user owns their registry session. The hint stays on one line because the
// footer that renders it budgets exactly one row.
const pushAuthHint = " — registry rejected these credentials; run `container registry login`, then retry"

// isPushAuthError reports whether a push error looks like a credentials
// problem. It reads only what the CLI printed: the full error text also carries
// the arguments, so an image whose own name contains "unauthorized" or
// "authentication" would otherwise be misdiagnosed.
func isPushAuthError(err error) bool {
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		return false
	}
	s := strings.ToLower(cliErr.Stderr)
	return strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "no credentials found") ||
		strings.Contains(s, "authentication")
}

// defaultRegistry is where a reference that names no registry host is pushed.
const defaultRegistry = "docker.io"

// PushTarget returns the registry host that pushing ref would publish to, so a
// confirmation can name the destination rather than only the image.
func PushTarget(ref string) string {
	head, rest, ok := strings.Cut(ref, "/")
	if !ok || rest == "" {
		return defaultRegistry
	}
	if head == "localhost" || strings.ContainsAny(head, ".:") {
		return head
	}
	return defaultRegistry
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

// ExactRef returns a reference that resolves to the selected image and reports
// whether one exists. An untagged row formats to a bare repository, which a
// registry resolves as ":latest" — a different artifact than the row shows — so
// actions that publish or archive an image must refuse it rather than guess.
func ExactRef(img Image) (string, bool) {
	if img.Repository == "" {
		return "", false
	}
	if img.Tag == "" || img.Tag == "<none>" {
		return "", false
	}
	return FormatRef(img), true
}

// FormatRef returns the full image reference "repo:tag".
func FormatRef(img Image) string {
	if img.Tag == "" || img.Tag == "<none>" {
		return img.Repository
	}
	return img.Repository + ":" + img.Tag
}
