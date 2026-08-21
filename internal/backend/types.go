package backend

import "time"

// Container represents a Mac container returned by the container CLI.
type Container struct {
	ID      string
	Name    string
	Image   string
	Status  string // "running", "exited", "created", ...
	Created time.Time
	Ports   []PortMapping
	Env     []string
	Labels  map[string]string

	// Hostname is the container's network hostname, when reported.
	Hostname string
	// Platform is the guest OS/architecture, e.g. "linux/arm64".
	Platform string
	// CPUs is the number of virtual CPUs configured for the container.
	CPUs int
	// MemoryBytes is the configured memory limit in bytes.
	MemoryBytes uint64
	Mounts      []Mount
	Networks    []Network
}

// Mount is a host→container volume mount.
type Mount struct {
	// Source identifies where the mount comes from, but is not always a path:
	// for a named volume it is the volume's display name, because the CLI
	// reports its source as the backing disk image rather than anything a
	// reader would recognise. Only a bind mount's Source is a host filesystem
	// path.
	Source      string
	Destination string
}

// Network is a container network attachment as reported by the live status.
type Network struct {
	Name string
	IP   string // ipv4 address, e.g. "192.168.64.2/24"
}

// PortMapping is a single host→container port binding.
type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string // "tcp" or "udp"
}

// Metrics holds real-time resource usage for a container.
type Metrics struct {
	ContainerID string
	CPUPercent  float64
	MemUsage    uint64 // bytes
	MemLimit    uint64 // bytes
}

// Image represents a container image.
type Image struct {
	ID         string
	Repository string
	Tag        string
	Size       int64
	Created    time.Time
}

// Volume represents a named volume.
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time

	// SizeBytes is the volume's storage quota in bytes.
	SizeBytes uint64
	// Format is the volume's filesystem format, e.g. "ext4".
	Format string
	Labels map[string]string
	// Options are the volume's configuration options.
	Options map[string]string
}

// ImageInspect carries the full inspection of a single image, including the
// per-platform manifest variants and the config of the manifest this host
// would run.
type ImageInspect struct {
	ID         string
	Repository string
	Tag        string
	Created    time.Time
	// Size is the size of the manifest variant this host would run.
	Size int64
	// Digest is the resolved digest of the manifest this host would run.
	Digest string
	// Platforms lists every platform variant in the image index.
	Platforms []ImagePlatform
	// Cmd/WorkingDir/Env describe the image's default run configuration for
	// the variant this host would run.
	Cmd        []string
	WorkingDir string
	Env        []string
	// LayerCount is the number of rootfs layers of the running variant.
	LayerCount int
}

// ImagePlatform is one per-OS/architecture manifest beneath a multi-arch index.
type ImagePlatform struct {
	OS           string
	Architecture string
	Variant      string
	Digest       string
	Size         int64
}

// VolumeInspect carries the full inspection of a single volume.
type VolumeInspect struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time
	SizeBytes  uint64
	Format     string
	Labels     map[string]string
	Options    map[string]string
}

// NetworkInfo represents a container network resource (as opposed to
// Network, a container's attachment to one).
type NetworkInfo struct {
	Name    string
	Mode    string
	Gateway string
	Subnet  string
	Created time.Time
}

// IsRunning reports whether the container is in the running state.
func (c Container) IsRunning() bool {
	return c.Status == "running"
}

// SystemStatus reports whether the container services are running, plus the
// identifying info the CLI reports.
type SystemStatus struct {
	// Status is the raw status string reported by the CLI, e.g. "running" or
	// "unregistered" when the services have never been started.
	Status      string
	Version     string
	AppRoot     string
	InstallRoot string
}

// IsRunning reports whether the container services are up.
func (s SystemStatus) IsRunning() bool {
	return s.Status == systemStatusRunning
}

// DiskUsage reports disk usage for containers, images and volumes.
type DiskUsage struct {
	Containers DiskUsageCategory
	Images     DiskUsageCategory
	Volumes    DiskUsageCategory
}

// DiskUsageCategory is the disk usage breakdown for one resource kind.
type DiskUsageCategory struct {
	Total            int
	Active           int
	SizeBytes        uint64
	ReclaimableBytes uint64
}
