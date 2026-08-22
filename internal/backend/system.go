package backend

import (
	"context"
	"encoding/json"
	"fmt"
)

// systemStatusRunning is the raw status string the CLI reports when the
// container services are up; any other value (e.g. "unregistered", reported
// when the services have never been started) means they are not.
const systemStatusRunning = "running"

// Apple container system status/df JSON (container 1.2.x).
type cliSystemStatus struct {
	APIServerVersion string `json:"apiServerVersion"`
	AppRoot          string `json:"appRoot"`
	InstallRoot      string `json:"installRoot"`
	Status           string `json:"status"`
}

type cliDiskUsageCategory struct {
	Active      int    `json:"active"`
	Reclaimable uint64 `json:"reclaimable"`
	SizeInBytes uint64 `json:"sizeInBytes"`
	Total       int    `json:"total"`
}

type cliDiskUsage struct {
	Containers cliDiskUsageCategory `json:"containers"`
	Images     cliDiskUsageCategory `json:"images"`
	Volumes    cliDiskUsageCategory `json:"volumes"`
}

// SystemStatus reports whether the container services are running. Unlike
// every other command this client runs, "system status" exits non-zero
// while still printing a valid, parseable JSON body (status: "unregistered")
// when the services have never been started - that is the normal down state
// this view exists to show, not a failure to report on. runRaw is used
// instead of runJSON so that body is not discarded on the non-zero exit; the
// exec error is only surfaced when there is no body to fall back on.
func (c *Client) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	out, runErr := c.runRaw(ctx, "system", "status", "--format", "json")
	var raw cliSystemStatus
	if err := json.Unmarshal(out, &raw); err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("system status: %w", runErr)
		}
		return nil, fmt.Errorf("system status: JSON decode: %w", err)
	}
	return mapSystemStatus(raw), nil
}

// DiskUsage returns disk usage for containers, images and volumes. Unlike
// SystemStatus, a down-services failure here has no JSON body to recover -
// the CLI prints a plain-text error instead - so any failed run is a real
// error for the caller to handle.
func (c *Client) DiskUsage(ctx context.Context) (*DiskUsage, error) {
	var raw cliDiskUsage
	if err := c.runJSON(ctx, &raw, "system", "df", "--format", "json"); err != nil {
		return nil, fmt.Errorf("system df: %w", err)
	}
	du := mapDiskUsage(raw)
	return &du, nil
}

func mapSystemStatus(r cliSystemStatus) *SystemStatus {
	return &SystemStatus{
		Status:      r.Status,
		Version:     r.APIServerVersion,
		AppRoot:     r.AppRoot,
		InstallRoot: r.InstallRoot,
	}
}

func mapDiskUsage(r cliDiskUsage) DiskUsage {
	return DiskUsage{
		Containers: mapDiskUsageCategory(r.Containers),
		Images:     mapDiskUsageCategory(r.Images),
		Volumes:    mapDiskUsageCategory(r.Volumes),
	}
}

func mapDiskUsageCategory(r cliDiskUsageCategory) DiskUsageCategory {
	return DiskUsageCategory{
		Total:            r.Total,
		Active:           r.Active,
		SizeBytes:        r.SizeInBytes,
		ReclaimableBytes: r.Reclaimable,
	}
}
