package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return v
}

func TestMapContainers_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliContainer](t, "list.json")
	got := mapContainers(raw)
	if len(got) == 0 {
		t.Fatal("expected at least one container from list.json")
	}
	found := false
	for _, c := range got {
		if c.Name == "vessel-probe" || c.ID == "vessel-probe" {
			found = true
			if c.Status != "running" {
				t.Errorf("vessel-probe status: want running, got %s", c.Status)
			}
			if c.Image == "" {
				t.Error("vessel-probe image empty")
			}
			if c.Created.IsZero() {
				t.Error("vessel-probe created empty")
			}
		}
		if c.Name == "vessel-ports" {
			if len(c.Ports) == 0 {
				t.Error("vessel-ports should have published ports")
			} else if c.Ports[0].HostPort != 8080 || c.Ports[0].ContainerPort != 80 {
				t.Errorf("vessel-ports ports wrong: %+v", c.Ports[0])
			}
			if c.Ports[0].Protocol != "tcp" {
				t.Errorf("protocol want tcp, got %s", c.Ports[0].Protocol)
			}
		}
	}
	if !found {
		t.Error("vessel-probe not found in mapped containers")
	}
}

func TestMapContainers_empty(t *testing.T) {
	got := mapContainers(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestMapContainers_mountsNetworksResources(t *testing.T) {
	raw := loadFixture[[]cliContainer](t, "container-mounts.json")
	got := mapContainers(raw)
	if len(got) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got))
	}
	c := got[0]
	if c.Hostname == "" {
		t.Error("hostname empty")
	}
	if c.Platform != "linux/arm64" {
		t.Errorf("platform want linux/arm64, got %q", c.Platform)
	}
	if c.CPUs != 4 {
		t.Errorf("cpus want 4, got %d", c.CPUs)
	}
	if c.MemoryBytes != 1073741824 {
		t.Errorf("memory want 1073741824, got %d", c.MemoryBytes)
	}
	if len(c.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(c.Mounts))
	}
	mt := c.Mounts[0]
	if mt.Destination != "/data" {
		t.Errorf("mount destination want /data, got %q", mt.Destination)
	}
	// A named volume's source is the backing disk image under Application
	// Support; the volume name is what identifies the mount to a reader.
	if mt.Source != "p2-live-probe" {
		t.Errorf("mount source want the volume name p2-live-probe, got %q", mt.Source)
	}
	if len(c.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(c.Networks))
	}
	net := c.Networks[0]
	if net.Name != "default" {
		t.Errorf("network name want default, got %q", net.Name)
	}
	if net.IP == "" || !strings.Contains(net.IP, ".") {
		t.Errorf("network ip missing or malformed: %q", net.IP)
	}
	if len(c.Ports) != 1 || c.Ports[0].HostPort != 18080 {
		t.Errorf("published ports wrong: %+v", c.Ports)
	}
}

func TestMapImages_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliImage](t, "images.json")
	got := mapImages(raw)
	if len(got) == 0 {
		t.Fatal("expected images")
	}
	img := got[0]
	if img.Repository == "" {
		t.Error("repository empty")
	}
	if img.ID == "" {
		t.Error("id empty")
	}
	ref := FormatRef(img)
	if ref == "" {
		t.Error("FormatRef empty")
	}
	if img.Size == raw[0].Configuration.Descriptor.Size {
		t.Errorf("Size is the index descriptor size (%d), not the platform image size", img.Size)
	}
	// Sizes of the linux manifests in the alpine index fixture.
	perArch := map[string]int64{"arm64": 4184689, "amd64": 3848024, "386": 3671765}
	if want, ok := perArch[runtime.GOARCH]; ok && img.Size != want {
		t.Errorf("Size on %s = %d, want %d", runtime.GOARCH, img.Size, want)
	}
}

// images-digest.json is real `container image list --format json` output
// captured after `image pull <repo>@sha256:…`: the CLI does emit digest-pinned
// names, so a list row must carry its pin through to FormatRef unchanged.
func TestMapImages_digestPinnedFixtureRoundTripsExactName(t *testing.T) {
	raw := loadFixture[[]cliImage](t, "images-digest.json")
	var pinned *cliImage
	for i := range raw {
		if strings.Contains(raw[i].Configuration.Name, refDigestSep) {
			pinned = &raw[i]
			break
		}
	}
	if pinned == nil {
		t.Fatal("fixture carries no digest-pinned name")
	}
	want := pinned.Configuration.Name
	got := mapImages([]cliImage{*pinned})[0]
	if got.Digest == "" {
		t.Fatal("mapped image lost the reference digest")
	}
	if got.Tag != "" {
		t.Errorf("pinned row must keep an empty tag, got %q", got.Tag)
	}
	if ref := FormatRef(got); ref != want {
		t.Fatalf("FormatRef must rebuild the exact CLI name:\n got %q\nwant %q", ref, want)
	}
}

func TestImageSize_fallsBackToDescriptorWithoutVariants(t *testing.T) {
	var r cliImage
	r.Configuration.Descriptor.Size = 9218
	if got := imageSize(r); got != 9218 {
		t.Errorf("imageSize = %d, want 9218", got)
	}
}

func TestImageSize_ignoresAttestationVariants(t *testing.T) {
	var r cliImage
	r.Configuration.Descriptor.Size = 9218
	r.Variants = []cliImageVariant{
		{Size: 86390},
		{Size: 3555096},
	}
	r.Variants[0].Platform.OS = "unknown"
	r.Variants[0].Platform.Architecture = "unknown"
	r.Variants[1].Platform.OS = guestOS
	r.Variants[1].Platform.Architecture = "some-other-arch"
	if got := imageSize(r); got != 3555096 {
		t.Errorf("imageSize = %d, want 3555096", got)
	}
}

func TestMapVolumes_fromLiveFixture(t *testing.T) {
	raw := loadFixture[[]cliVolume](t, "volumes.json")
	got := mapVolumes(raw)
	if len(got) == 0 {
		t.Fatal("expected volumes")
	}
	if got[0].Name == "" {
		t.Error("volume name empty")
	}
	if got[0].Driver == "" {
		t.Error("volume driver empty")
	}
}

func TestFormatPorts(t *testing.T) {
	if FormatPorts(nil) != "-" {
		t.Fatal("nil ports")
	}
	ports := []PortMapping{
		{HostPort: 80, ContainerPort: 8080},
		{HostPort: 443, ContainerPort: 8443},
	}
	want := "80→8080, 443→8443"
	if got := FormatPorts(ports); got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestIsRunning(t *testing.T) {
	c := Container{Status: "running"}
	if !c.IsRunning() {
		t.Fatal("running")
	}
	c.Status = "stopped"
	if c.IsRunning() {
		t.Fatal("stopped")
	}
}

func TestSplitRef_taggedReference(t *testing.T) {
	repo, tag, digest := splitRef("docker.io/library/alpine:latest")
	if repo != "docker.io/library/alpine" || tag != "latest" || digest != "" {
		t.Fatalf("got %q %q %q", repo, tag, digest)
	}
}

func TestSplitRef_nameOnlyDefaultsLatestTag(t *testing.T) {
	repo, tag, digest := splitRef("alpine")
	if repo != "alpine" || tag != "latest" || digest != "" {
		t.Fatalf("got %q %q %q", repo, tag, digest)
	}
}

func TestSplitRef_registryPortIsNotATag(t *testing.T) {
	repo, tag, digest := splitRef("registry.local:5000/team/app")
	if repo != "registry.local:5000/team/app" || tag != "latest" || digest != "" {
		t.Fatalf("got %q %q %q", repo, tag, digest)
	}
}

func TestSplitRef_digestPinnedKeepsDigestAndEmptyTag(t *testing.T) {
	repo, tag, digest := splitRef("docker.io/library/alpine@sha256:abc123")
	if repo != "docker.io/library/alpine" || tag != "" || digest != "sha256:abc123" {
		t.Fatalf("got %q %q %q", repo, tag, digest)
	}
}

func TestSplitRef_taggedDigestPinnedKeepsBoth(t *testing.T) {
	repo, tag, digest := splitRef("registry.local:5000/team/app:v1@sha256:abc123")
	if repo != "registry.local:5000/team/app" || tag != "v1" || digest != "sha256:abc123" {
		t.Fatalf("got %q %q %q", repo, tag, digest)
	}
}

func TestFormatRef_digestPinnedRoundTripsByteForByte(t *testing.T) {
	const pinned = "docker.io/library/alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b"
	repo, tag, digest := splitRef(pinned)
	img := Image{Repository: repo, Tag: tag, Digest: digest}
	if got := FormatRef(img); got != pinned {
		t.Fatalf("digest ref must survive FormatRef exactly:\n got %q\nwant %q", got, pinned)
	}
}

func TestFormatRef_taggedDigestRoundTripsByteForByte(t *testing.T) {
	const pinned = "registry.local:5000/team/app:v1@sha256:abc123"
	repo, tag, digest := splitRef(pinned)
	img := Image{Repository: repo, Tag: tag, Digest: digest}
	if got := FormatRef(img); got != pinned {
		t.Fatalf("tagged digest ref must survive FormatRef exactly:\n got %q\nwant %q", got, pinned)
	}
}

func TestFormatRef_nonDigestReferencesUnchanged(t *testing.T) {
	if got := FormatRef(Image{Repository: "alpine", Tag: latestRef}); got != "alpine:latest" {
		t.Fatalf("tag-only ref changed: %q", got)
	}
	if got := FormatRef(Image{Repository: "alpine", Tag: untaggedMarker}); got != "alpine" {
		t.Fatalf("<none>-tagged ref changed: %q", got)
	}
	if got := FormatRef(Image{Repository: "alpine"}); got != "alpine" {
		t.Fatalf("untagged ref changed: %q", got)
	}
}

func TestCreatedParse(t *testing.T) {
	raw := []cliContainer{{
		ID: "x",
		Configuration: cliContainerConfig{
			ID:           "x",
			CreationDate: "2026-08-05T16:08:35Z",
			Image:        cliImageRef{Reference: "alpine:latest"},
		},
		Status: cliContainerStatus{State: "running"},
	}}
	got := mapContainers(raw)
	want := time.Date(2026, 8, 5, 16, 8, 35, 0, time.UTC)
	if !got[0].Created.Equal(want) {
		t.Fatalf("want %v got %v", want, got[0].Created)
	}
}

// A bind mount carries no volume name, so its host path is the only identity
// it has and must survive.
func TestMapContainers_bindMountKeepsHostPath(t *testing.T) {
	raw := loadFixture[[]cliContainer](t, "container-mounts.json")
	raw[0].Configuration.Mounts[0].Type.Volume.Name = ""
	raw[0].Configuration.Mounts[0].Source = "/Users/me/project"

	got := mapContainers(raw)
	if len(got) != 1 || len(got[0].Mounts) != 1 {
		t.Fatalf("expected 1 container with 1 mount, got %+v", got)
	}
	if src := got[0].Mounts[0].Source; src != "/Users/me/project" {
		t.Errorf("bind mount source want /Users/me/project, got %q", src)
	}
}

// A stopped container reports an empty status.networks, but the network it is
// configured on is still known. `container list --all` includes stopped rows,
// so the pane must not lose the network name just because the container is
// not running.
func TestMapContainers_stoppedContainerKeepsConfiguredNetwork(t *testing.T) {
	raw := loadFixture[[]cliContainer](t, "container-stopped.json")
	if len(raw) != 1 {
		t.Fatalf("expected 1 container in the fixture, got %d", len(raw))
	}
	if len(raw[0].Status.Networks) != 0 {
		t.Fatalf("fixture no longer models a stopped container: status.networks = %+v", raw[0].Status.Networks)
	}

	got := mapContainers(raw)
	if len(got) != 1 {
		t.Fatalf("expected 1 container, got %d", len(got))
	}
	c := got[0]
	if c.Status != "stopped" {
		t.Errorf("status want stopped, got %q", c.Status)
	}
	if len(c.Networks) != 1 {
		t.Fatalf("expected the configured network to survive, got %+v", c.Networks)
	}
	if c.Networks[0].Name != "default" {
		t.Errorf("network name want default, got %q", c.Networks[0].Name)
	}
	// A stopped container has no runtime address, so only the name renders.
	if c.Networks[0].IP != "" {
		t.Errorf("stopped container reported an ip: %q", c.Networks[0].IP)
	}
	if got := FormatNetworks(c.Networks); got != "default" {
		t.Errorf("FormatNetworks = %q, want %q", got, "default")
	}
}

// The running case must keep its address rather than being replaced by the
// bare configured name.
func TestMapContainers_runningContainerKeepsStatusAddress(t *testing.T) {
	raw := loadFixture[[]cliContainer](t, "container-mounts.json")
	got := mapContainers(raw)
	if len(got) != 1 || len(got[0].Networks) != 1 {
		t.Fatalf("expected 1 container with 1 network, got %+v", got)
	}
	net := got[0].Networks[0]
	if net.Name != "default" || net.IP == "" {
		t.Errorf("running container network = %+v, want default with an ip", net)
	}
}
