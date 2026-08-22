package doctor

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const realVersionBanner = "container CLI version 1.2.2 (build: release, commit: unspeci)"

// healthyRunner answers every command doctor asks on a healthy machine:
// container 1.2.2 with services up, macOS 26. Dispatch keys on arguments
// because doctor invokes the LookPath-resolved path, not the bare name.
func healthyRunner(name string, args ...string) ([]byte, error) {
	switch {
	case len(args) == 1 && args[0] == "--version":
		return []byte(realVersionBanner + "\n"), nil
	case len(args) == 2 && args[0] == "system" && args[1] == "status":
		return []byte("* vessel api-server is running\n"), nil
	case name == macOSVersionCmd:
		return []byte("26.5.2\n"), nil
	default:
		return nil, fmt.Errorf("fake runner: unexpected call %q %v", name, args)
	}
}

// stubDoctor points every seam at a healthy machine so Run() never touches
// the host. Tests override single seams afterwards; cleanup restores all of
// them to their originals regardless.
func stubDoctor(t *testing.T) *bytes.Buffer {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	buf := &bytes.Buffer{}
	savedStdout, savedLookPath, savedRunCmd, savedArch := stdout, lookPath, runCmd, hostArch
	t.Cleanup(func() {
		stdout, lookPath, runCmd, hostArch = savedStdout, savedLookPath, savedRunCmd, savedArch
	})

	stdout = buf
	hostArch = supportedArch
	lookPath = func(name string) (string, error) { return "/fake/bin/" + name, nil }
	runCmd = healthyRunner
	return buf
}

func assertExitAndOutput(t *testing.T, buf *bytes.Buffer, wantCode int, wantSubstrings ...string) {
	t.Helper()
	code := Run()
	if code != wantCode {
		t.Fatalf("doctor exit code = %d, want %d\noutput:\n%s", code, wantCode, buf)
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("doctor output missing %q:\n%s", want, buf)
		}
	}
}

func TestParseSemver_realCLIBanner(t *testing.T) {
	v, ok := parseSemver(realVersionBanner)

	if !ok {
		t.Fatal("parseSemver rejected the real container --version banner")
	}
	if v != (semver{1, 2, 2}) {
		t.Fatalf("parseSemver(banner) = %v, want 1.2.2", v)
	}
}

func TestParseSemver_twoComponentVersion(t *testing.T) {
	v, ok := parseSemver("container CLI version 1.2")

	if !ok {
		t.Fatal("parseSemver rejected a two-component version")
	}
	if v != (semver{1, 2, 0}) {
		t.Fatalf("parseSemver(1.2) = %v, want 1.2.0", v)
	}
}

func TestParseSemver_rejectsGarbage(t *testing.T) {
	if _, ok := parseSemver("banana"); ok {
		t.Error("parseSemver accepted text without a dotted number")
	}
	if _, ok := parseSemver(""); ok {
		t.Error("parseSemver accepted empty output")
	}
}

func TestCompareSemvar_ordersTriples(t *testing.T) {
	floor := semver{1, 2, 0}

	if c := compareSemver(semver{1, 1, 9}, floor); c >= 0 {
		t.Errorf("1.1.9 vs 1.2.0 compare = %d, want negative", c)
	}
	if c := compareSemver(floor, floor); c != 0 {
		t.Errorf("equal triples compare = %d, want 0", c)
	}
	if c := compareSemver(semver{1, 2, 1}, floor); c <= 0 {
		t.Errorf("1.2.1 vs 1.2.0 compare = %d, want positive", c)
	}
	if c := compareSemver(semver{1, 10, 0}, floor); c <= 0 {
		t.Errorf("1.10.0 vs 1.2.0 compare = %d, want positive (numeric, not string order)", c)
	}
}

func TestMajorVersion_readsProductVersion(t *testing.T) {
	major, ok := majorVersion("26.5.2")

	if !ok || major != requiredMacOSMajor {
		t.Fatalf("majorVersion(26.5.2) = (%d, %v), want (%d, true)", major, ok, requiredMacOSMajor)
	}
	if _, ok := majorVersion("unknown"); ok {
		t.Error("majorVersion accepted garbage")
	}
}

func TestRun_healthyEnvironmentPasses(t *testing.T) {
	buf := stubDoctor(t)

	assertExitAndOutput(t, buf, 0,
		"container version: 1.2.2",
		"macOS version: 26.5.2",
		"architecture: arm64",
		"\nok\n",
	)
}

func TestRun_oldContainerCLIFails(t *testing.T) {
	buf := stubDoctor(t)
	runCmd = func(name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--version" {
			return []byte("container CLI version 1.1.4 (build: release, commit: unspeci)\n"), nil
		}
		return healthyRunner(name, args...)
	}

	assertExitAndOutput(t, buf, 1,
		"container version: 1.1.4 BELOW MINIMUM 1.2.0",
		"problems detected",
	)
}

func TestRun_unparseableContainerVersionFails(t *testing.T) {
	buf := stubDoctor(t)
	runCmd = func(name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--version" {
			return []byte("some future banner with no numbers\n"), nil
		}
		return healthyRunner(name, args...)
	}

	assertExitAndOutput(t, buf, 1, "container version: UNPARSEABLE")
}

func TestRun_intelHostFails(t *testing.T) {
	buf := stubDoctor(t)
	hostArch = "amd64"

	assertExitAndOutput(t, buf, 1,
		"architecture: amd64 UNSUPPORTED (arm64 required)",
		"problems detected",
	)
}

func TestRun_macOSBelowFloorFails(t *testing.T) {
	buf := stubDoctor(t)
	runCmd = func(name string, args ...string) ([]byte, error) {
		if name == macOSVersionCmd {
			return []byte("14.6.1\n"), nil
		}
		return healthyRunner(name, args...)
	}

	assertExitAndOutput(t, buf, 1, "macOS version: 14.6.1 UNSUPPORTED (macOS 26+ required)")
}

func TestRun_legacyMacOSWarnsButPasses(t *testing.T) {
	buf := stubDoctor(t)
	runCmd = func(name string, args ...string) ([]byte, error) {
		if name == macOSVersionCmd {
			return []byte("15.3\n"), nil
		}
		return healthyRunner(name, args...)
	}

	assertExitAndOutput(t, buf, 0,
		"macOS version: 15.3 LIMITED (macOS 26+ recommended)",
		"\nok\n",
	)
}

func TestRun_unparseableMacOSFails(t *testing.T) {
	buf := stubDoctor(t)
	runCmd = func(name string, args ...string) ([]byte, error) {
		if name == macOSVersionCmd {
			return []byte("not-a-version\n"), nil
		}
		return healthyRunner(name, args...)
	}

	assertExitAndOutput(t, buf, 1, "macOS version: UNPARSEABLE")
}

func TestRun_missingCLISkipsVersionProbe(t *testing.T) {
	buf := stubDoctor(t)
	lookPath = func(name string) (string, error) {
		return "", errors.New("exec: not found")
	}
	var probes []string
	runCmd = func(name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--version" {
			probes = append(probes, name)
		}
		if name == macOSVersionCmd {
			return []byte("26.5.2\n"), nil
		}
		return nil, fmt.Errorf("fake runner: unexpected call %q %v", name, args)
	}

	code := Run()

	if code != 1 {
		t.Fatalf("doctor exit code = %d, want 1\noutput:\n%s", code, buf)
	}
	if len(probes) != 0 {
		t.Errorf("--version probed %d time(s) despite missing CLI", len(probes))
	}
	if out := buf.String(); strings.Contains(out, "container version:") {
		t.Errorf("version line printed without a CLI:\n%s", out)
	}
}
