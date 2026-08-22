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
// found, so callers should pass the reference. Like `volume inspect`, the
// command prints JSON by default and --format json is not an accepted flag:
// `container image inspect --format json` fails with "Unknown option --format"
// on container 1.2.2, so the flag must not be added back.
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
// The batched invocation shares one budget across every target, so it gets
// the confirmed-removal window rather than the quick default.
func (c *Client) RemoveImage(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return errNoDeleteTargets
	}
	_, err := c.runWithTimeout(ctx, confirmTimeout, append([]string{"image", "delete"}, ids...)...)
	return err
}

// PullImage pulls an image (blocking until complete). A pull is an image
// transfer like save/load/push — and `container run` holds this same budget
// precisely because it may pull a missing image first — so it runs under the
// long transfer budget rather than the quick default.
func (c *Client) PullImage(ctx context.Context, ref string) error {
	_, err := c.runWithTimeout(ctx, globalTimeout, "image", "pull", ref)
	return err
}

// PruneImages removes unused images. A prune sweeps the whole store, so it
// runs under the long sweep budget rather than the quick default.
func (c *Client) PruneImages(ctx context.Context) error {
	_, err := c.runWithTimeout(ctx, globalTimeout, "image", "prune")
	return err
}

// TagImage creates a new reference for an existing image. Tagging itself is
// quick, but it travels with the transfer verbs whose budget it shares.
func (c *Client) TagImage(ctx context.Context, ref, newRef string) error {
	_, err := c.runWithTimeout(ctx, globalTimeout, "image", "tag", ref, newRef)
	return err
}

// SaveImage writes one image to an OCI-compatible tar archive at path. A
// large image streams at a fixed rate, so the transfer runs under the long
// transfer budget; the quick default would kill it mid-write.
func (c *Client) SaveImage(ctx context.Context, ref, path string) error {
	path, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("save image: %w", err)
	}
	_, err = c.runWithTimeout(ctx, globalTimeout, "image", "save", "--output", path, ref)
	return err
}

// LoadImage imports images from an OCI-compatible tar archive at path.
// The path is checked up front so a missing file reports plainly instead of
// surfacing as a raw CLI error. Like SaveImage it streams a potentially
// large archive, so it shares the long transfer budget.
func (c *Client) LoadImage(ctx context.Context, path string) error {
	path, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	_, err = c.runWithTimeout(ctx, globalTimeout, "image", "load", "--input", path)
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

// PushImage pushes an image to its registry. Uploading a large image is the
// slowest verb here, so it shares the long transfer budget.
func (c *Client) PushImage(ctx context.Context, ref string) error {
	_, err := c.runWithTimeout(ctx, globalTimeout, "image", "push", ref)
	switch pushDenialOf(err) {
	case denialCredentials:
		return &HintedError{Err: err, Hint: pushAuthHint}
	case denialPermission:
		return &HintedError{Err: err, Hint: pushPermissionHint}
	}
	return err
}

// HintedError carries a refused verb's hint beside its raw failure instead of
// baked into one string. The raw CLI error alone is longer than any footer
// row, so a hint appended at its tail could never survive truncation; keeping
// the two apart lets the footer budget them separately - truncate the error,
// reserve room for the whole hint - while Error still reads as one message for
// logs and callers that only print.
type HintedError struct {
	Err  error
	Hint string
}

func (e *HintedError) Error() string { return e.Err.Error() + e.Hint }

func (e *HintedError) Unwrap() error { return e.Err }

// PushAuthNotice and PushPermissionNotice are what the images detail pane shows
// after a refused push. Container registry login is intentionally out of
// vessel's scope: the user owns their registry session. Both lead with the
// verdict and carry no more prose than that, because the smallest supported pane
// (18x4 at 60x12 with the command log open) holds roughly four wrapped rows.
// PushPermissionNotice already wraps to exactly four rows there, saturating that
// budget with no slack: it leads the pane, so it renders in full today, but any
// reword that makes it longer is clipped rather than merely crowding the fields
// below it. Measure a reword against that geometry, not against the terminal.
const (
	PushAuthNotice = "push rejected — run `container registry login`"
	// A 403 does not on its own establish that the session holds valid,
	// insufficiently-privileged credentials: Google Artifact Registry and
	// Docker Hub both answer an *unauthenticated* push with 403 rather than
	// 401, and the distribution wording is identical either way. So the notice
	// names both possibilities instead of telling a logged-out user that the
	// one thing they haven't tried is the one thing that will not help.
	PushPermissionNotice = "push forbidden — may lack write access or need login"
)

// The fuller instructions appended to the error itself, which the footer
// renders. Each stays on one line because the footer budgets exactly one row.
const (
	pushAuthHint       = " — registry rejected these credentials; run `container registry login`, then retry"
	pushPermissionHint = " — registry refused the push: this account may lack write access, or you may need to log in with `container registry login`"
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

// Reference syntax pieces shared by splitRef and FormatRef. Keeping them named
// is what lets both sides rebuild exactly what the other parsed.
const (
	refTagSep      = ":"
	refDigestSep   = "@"
	latestRef      = "latest"
	untaggedMarker = "<none>"
)

func mapImages(raw []cliImage) []Image {
	out := make([]Image, 0, len(raw))
	for _, r := range raw {
		repo, tag, digest := splitRef(r.Configuration.Name)
		img := Image{
			ID:         r.ID,
			Repository: repo,
			Tag:        tag,
			Digest:     digest,
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
	repo, tag, _ := splitRef(r.Configuration.Name)
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

// splitRef breaks a CLI image name into repository, tag and reference digest.
// A digest-pinned name ("repo@sha256:…", or "repo:tag@sha256:…") keeps its
// digest so FormatRef can rebuild the exact reference; dropping it made every
// caller act on whatever artifact the bare repository resolves to instead of
// the pinned one. A name-only reference defaults its tag to "latest", matching
// what a registry resolves it as; a digest-pinned name keeps an empty tag
// because the pin, not a tag, identifies the artifact.
func splitRef(name string) (repo, tag, digest string) {
	if name == "" {
		return "", "", ""
	}
	// Everything before "@" may still carry a tag; everything after is the pin.
	if base, pin, ok := strings.Cut(name, refDigestSep); ok {
		repo, tag = splitRepoTag(base)
		return repo, tag, pin
	}
	repo, tag = splitRepoTag(name)
	if tag == "" {
		tag = latestRef
	}
	return repo, tag, ""
}

// splitRepoTag splits "repo[:tag]" at the last ':' that looks like a tag — not
// a port in "host:5000/repo", which contains '/' after the colon. The tag comes
// back empty when absent; whether that means "latest" is the caller's policy.
func splitRepoTag(name string) (repo, tag string) {
	i := strings.LastIndex(name, refTagSep)
	if i < 0 {
		return name, ""
	}
	if strings.Contains(name[i+len(refTagSep):], "/") {
		return name, ""
	}
	return name[:i], name[i+1:]
}

// ExactRef returns a reference that resolves to the selected image and reports
// whether one exists. An untagged, unpinned row formats to a bare repository,
// which a registry resolves as ":latest" — a different artifact than the row
// shows — so actions that publish or archive an image must refuse it rather
// than guess. A digest-pinned row is refused too, tag or no tag: it formats
// exactly, but the CLI's acceptance of digest-pinned sources for tag, save and
// push is unverified, and the refusal is the conservative side of that.
func ExactRef(img Image) (string, bool) {
	if img.Repository == "" {
		return "", false
	}
	if img.Digest != "" {
		return "", false
	}
	if !hasNamedTag(img) {
		return "", false
	}
	return FormatRef(img), true
}

// hasNamedTag reports whether the row carries a tag that names an artifact.
// The CLI writes untaggedMarker for a row that lost its tag, which addresses
// nothing, so it counts as absent alongside an empty tag.
func hasNamedTag(img Image) bool {
	return img.Tag != "" && img.Tag != untaggedMarker
}

// FormatRef returns the full image reference: "repo:tag", or the exact pinned
// reference "repo[:tag]@digest" when the image carries one. Tag-only and
// name-only references are formatted exactly as before the digest branch
// existed — it fires only on a non-empty Digest.
func FormatRef(img Image) string {
	ref := img.Repository
	if hasNamedTag(img) {
		ref += refTagSep + img.Tag
	}
	if img.Digest != "" {
		ref += refDigestSep + img.Digest
	}
	return ref
}
