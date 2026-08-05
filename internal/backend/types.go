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
}

// IsRunning reports whether the container is in the running state.
func (c Container) IsRunning() bool {
	return c.Status == "running"
}
