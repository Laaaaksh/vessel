package backend

import (
	"context"
	"fmt"
	"strings"
)

// Flags for `container run`, named so the practical flag subset the run form
// exposes is not scattered as magic strings across the package. This is a
// deliberate subset of the CLI's full run surface: port mapping, env,
// volumes, memory, cpus, detached, tty/interactive, name and arch. Flags like
// --cap-add, --ulimit, --uid or --env-file are out of scope.
const (
	runFlagDetach      = "-d"
	runFlagTTY         = "-t"
	runFlagInteractive = "-i"
	runFlagName        = "--name"
	runFlagMemory      = "-m"
	runFlagCPUs        = "-c"
	runFlagArch        = "--arch"
	runFlagPublish     = "-p"
	runFlagEnv         = "-e"
	runFlagVolume      = "-v"
)

// defaultShell is the fallback interactive/exec shell when the user has not
// configured one, matched to what container images ship almost universally.
const defaultShellPath = "/bin/sh"

var (
	errEmptyRunImage    = fmt.Errorf("run requires an image")
	errEmptyExecCommand = fmt.Errorf("exec requires a command")
)

// RunOptions is the practical subset of `container run` flags the run form
// exposes. Every field is optional except by way of Run's image argument.
type RunOptions struct {
	Name    string
	Ports   []string // "[host-ip:]host-port:container-port[/proto]", passed through verbatim
	Env     []string // "KEY=VALUE"
	Volumes []string // "host:container"
	Memory  string
	CPUs    string
	Arch    string

	Detached    bool
	TTY         bool
	Interactive bool
}

// Run starts a container from image with the given options and returns the
// CLI's stdout, trimmed (the container ID for a detached run). It holds the
// transfer/sweep budget rather than the quick default: `container run` pulls a
// missing image and boots a VM before it returns, so it belongs to the
// known-long set alongside the image transfers.
func (c *Client) Run(ctx context.Context, image string, opts RunOptions) (string, error) {
	if image == "" {
		return "", errEmptyRunImage
	}
	args := []string{"run"}
	if opts.Detached {
		args = append(args, runFlagDetach)
	}
	if opts.TTY {
		args = append(args, runFlagTTY)
	}
	if opts.Interactive {
		args = append(args, runFlagInteractive)
	}
	if opts.Name != "" {
		args = append(args, runFlagName, opts.Name)
	}
	if opts.Memory != "" {
		args = append(args, runFlagMemory, opts.Memory)
	}
	if opts.CPUs != "" {
		args = append(args, runFlagCPUs, opts.CPUs)
	}
	if opts.Arch != "" {
		args = append(args, runFlagArch, opts.Arch)
	}
	for _, p := range opts.Ports {
		args = append(args, runFlagPublish, p)
	}
	for _, e := range opts.Env {
		args = append(args, runFlagEnv, e)
	}
	for _, v := range opts.Volumes {
		args = append(args, runFlagVolume, v)
	}
	args = append(args, image)
	out, err := c.runWithTimeout(ctx, globalTimeout, args...)
	return strings.TrimSpace(string(out)), err
}

// Exec runs a one-shot command inside a running container via the container's
// shell and returns its combined output, trimmed. Unlike ShellCmd (an
// interactive TTY attach driven by tea.ExecProcess), Exec blocks until the
// command exits, so it is for a single command rather than a session. The
// command is user-authored and so gets execTimeout rather than the quick
// default, which would kill an ordinary package install mid-run.
func (c *Client) Exec(ctx context.Context, id, shell, command string) (string, error) {
	if command == "" {
		return "", errEmptyExecCommand
	}
	out, err := c.runWithTimeout(ctx, execTimeout, "exec", id, resolveShell(shell), "-c", command)
	return strings.TrimSpace(string(out)), err
}

// resolveShell applies the same empty-shell fallback ShellCmd and Exec both
// need, so an unconfigured shell behaves identically for interactive attach
// and one-shot exec.
func resolveShell(shell string) string {
	if shell == "" {
		return defaultShellPath
	}
	return shell
}
