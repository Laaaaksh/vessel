package backend

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// argvOf splits Run/Exec's captured output back into the exact argv the fake
// CLI received: fakecli's run/exec cases print one argument per line (never
// word-split), so this proves argument boundaries survive even when a value
// contains a literal space - the whole point of the prompt-input fix this
// feature depends on.
func argvOf(out string) []string {
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func TestRun_minimalOptions_justRunsTheImage(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := c.Run(ctx, "alpine", RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpine"}
	if got := argvOf(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestRun_fullOptions_buildsFlagsInDeclaredOrder(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := RunOptions{
		Name:        "web",
		Memory:      "512M",
		CPUs:        "2",
		Arch:        "arm64",
		Detached:    true,
		TTY:         true,
		Interactive: true,
		Ports:       []string{"8080:80"},
		Env:         []string{"FOO=bar"},
		Volumes:     []string{"data:/var/data"},
	}
	out, err := c.Run(ctx, "nginx:latest", opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-d", "-t", "-i",
		"--name", "web",
		"-m", "512M",
		"-c", "2",
		"--arch", "arm64",
		"-p", "8080:80",
		"-e", "FOO=bar",
		"-v", "data:/var/data",
		"nginx:latest",
	}
	if got := argvOf(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestRun_multiplePortsEnvVolumes_eachFlagRepeated(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := RunOptions{
		Ports:   []string{"8080:80", "8443:443"},
		Env:     []string{"A=1", "B=2"},
		Volumes: []string{"data:/data", "cache:/cache"},
	}
	out, err := c.Run(ctx, "nginx", opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-p", "8080:80", "-p", "8443:443",
		"-e", "A=1", "-e", "B=2",
		"-v", "data:/data", "-v", "cache:/cache",
		"nginx",
	}
	if got := argvOf(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

// A value containing a space (env var, volume path) must reach the CLI intact
// as one argument, not split on whitespace - proving Run passes the form's
// values straight through instead of re-tokenizing them.
func TestRun_valueWithSpace_passedAsSingleArgument(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := RunOptions{
		Volumes: []string{"my data:/var/my data"},
		Env:     []string{"GREETING=hello there"},
	}
	out, err := c.Run(ctx, "alpine", opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-e", "GREETING=hello there",
		"-v", "my data:/var/my data",
		"alpine",
	}
	if got := argvOf(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestRun_emptyImage_refusesWithoutRunning(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Run(ctx, "", RunOptions{}); !errors.Is(err, errEmptyRunImage) {
		t.Fatalf("Run() error = %v, want errEmptyRunImage", err)
	}
	if log := c.CommandLog(); len(log) != 0 {
		t.Fatalf("no command should be issued, got %v", log)
	}
}

func TestExec_defaultShell_capturesOutput(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := c.Exec(ctx, "web", "", "ls -la /data")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"web", "/bin/sh", "-c", "ls -la /data"}
	if got := argvOf(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestExec_configuredShell_overridesDefault(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := c.Exec(ctx, "web", "/bin/bash", "echo hi")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"web", "/bin/bash", "-c", "echo hi"}
	if got := argvOf(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestExec_emptyCommand_refusesWithoutRunning(t *testing.T) {
	c := NewClientWithBinary(fakeBinary(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Exec(ctx, "web", "", ""); !errors.Is(err, errEmptyExecCommand) {
		t.Fatalf("Exec() error = %v, want errEmptyExecCommand", err)
	}
	if log := c.CommandLog(); len(log) != 0 {
		t.Fatalf("no command should be issued, got %v", log)
	}
}
