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
	switch pushDenialOf(err) {
	case denialCredentials:
		return fmt.Errorf("%w%s", err, pushAuthHint)
	case denialPermission:
		return fmt.Errorf("%w%s", err, pushPermissionHint)
	}
	return err
}

// PushAuthNotice and PushPermissionNotice are what the images detail pane shows
// after a refused push. Container registry login is intentionally out of
// vessel's scope: the user owns their registry session. Both lead with the
// verdict and carry no more prose than that, because the smallest supported pane
// (18x4 at 60x12 with the command log open) holds roughly three wrapped rows.
const (
	PushAuthNotice = "push rejected — run `container registry login`"
	// A 403 usually means the session is valid but the account cannot write
	// here, so repeating the login it already holds would send the user in a
	// circle. Known limitation: that premise is not universal — Google Artifact
	// Registry and Docker Hub both answer an *unauthenticated* push with 403
	// rather than 401, and the distribution wording is identical either way, so
	// a logged-out user pushing there is told login will not help when it is
	// exactly what they need. The blunter wording is a deliberate simplification
	// held for the smallest pane, and is not yet filed as follow-up.
	PushPermissionNotice = "push forbidden — no write access; login won't help"
)

// The fuller instructions appended to the error itself, which the footer
// renders. Each stays on one line because the footer budgets exactly one row.
const (
	pushAuthHint       = " — registry rejected these credentials; run `container registry login`, then retry"
	pushPermissionHint = " — registry refused the push: this account has no write access to that repository, so `container registry login` again will not help"
)

// Multi-word phrases only a registry emits. Matching bare words would misread
// the reference: the CLI echoes it into stderr, so pushing
// myorg/authentication-service:v1 would classify any failure as a credentials
// problem. A reference cannot contain a space, so a phrase cannot collide.
var (
	permissionStderrPhrases = []string{
		"403 forbidden",
	}
	credentialStderrPhrases = []string{
		"401 unauthorized",
		"no credentials found",
		"authentication required",
		"unauthorized: authentication",
	}
)

type pushDenial int

const (
	denialNone pushDenial = iota
	denialCredentials
	denialPermission
)

// pushDenialOf reports how the registry refused a push, reading only what the
// CLI printed and never the arguments it was handed. Permission is checked
// first: a 403 is the more specific verdict when both shapes appear.
func pushDenialOf(err error) pushDenial {
	var cliErr *CLIError
	if err == nil || !errors.As(err, &cliErr) {
		return denialNone
	}
	s := strings.ToLower(cliErr.Stderr)
	for _, phrase := range permissionStderrPhrases {
		if strings.Contains(s, phrase) {
			return denialPermission
		}
	}
	for _, phrase := range credentialStderrPhrases {
		if strings.Contains(s, phrase) {
			return denialCredentials
		}
	}
	return denialNone
}

// PushDenialNotice returns the images-panel notice matching how the registry
// refused a push, or "" when the failure was not a refusal.
func PushDenialNotice(err error) string {
	switch pushDenialOf(err) {
	case denialCredentials:
		return PushAuthNotice
	case denialPermission:
		return PushPermissionNotice
	}
	return ""
}

// PushTarget returns the registry host that pushing ref publishes to, and
// reports whether the reference names one at all. An unqualified reference goes
// to whatever the CLI has configured as its default registry, which vessel does
// not read — so callers must say nothing rather than assert a guess, since the
// push confirmation exists to name the real destination.
func PushTarget(ref string) (string, bool) {
	head, rest, ok := strings.Cut(ref, "/")
	if !ok || rest == "" {
		return "", false
	}
	if head == "localhost" || strings.ContainsAny(head, ".:") {
		return head, true
	}
	return "", false
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
