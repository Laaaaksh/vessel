package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/Laaaaksh/vessel/internal/config"
)

// Documented support floor. README Requirements asks for macOS 26+ on Apple
// silicon ("macOS 15 may work with limitations") plus the Apple container CLI;
// docs/APPLE_CONTAINER_MATRIX.md probes container 1.2.x and was verified live
// on 1.2.2, so anything older than 1.2.0 is untested territory.
const (
	containerBinary = "container"
	macOSVersionCmd = "sw_vers"
	clipboardBinary = "pbcopy"
	supportedArch   = "arm64"

	requiredMacOSMajor = 26
	warnMacOSMajor     = 15
)

var minContainerVersion = semver{1, 2, 0}

// Test seams: run shells out to the real machine through these, so tests can
// fake a broken or outdated environment without touching the host.
var (
	stdout   io.Writer = os.Stdout
	lookPath           = exec.LookPath
	runCmd             = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	}
	hostArch = runtime.GOARCH
)

// Run prints environment diagnostics to stdout and returns a process exit code.
func Run() int {
	r := newReport(stdout)
	r.linef("vessel doctor\n")
	r.linef("-------------\n")

	checkContainerCLI(r)
	checkPlatform(r)
	checkConfig(r)
	checkClipboard(r)

	if r.ok {
		r.linef("\nok\n")
		return 0
	}
	r.linef("\nproblems detected\n")
	return 1
}

// report accumulates the doctor's findings: every line prints as it is
// discovered, and any failure flips the exit code once at the end.
type report struct {
	w  io.Writer
	ok bool
}

func newReport(w io.Writer) *report { return &report{w: w, ok: true} }

func (r *report) linef(format string, args ...any) {
	// Printing is the doctor's only output channel; there is nowhere more
	// useful to report a write failure than the diagnostics themselves.
	_, _ = fmt.Fprintf(r.w, format, args...)
}

func (r *report) failf(format string, args ...any) {
	r.ok = false
	r.linef(format, args...)
}

func checkContainerCLI(r *report) {
	path, err := lookPath(containerBinary)
	if err != nil {
		r.failf("container CLI: MISSING (%v)\n", err)
		return
	}
	r.linef("container CLI: %s\n", path)
	checkContainerVersion(r, path)
	checkSystemStatus(r, path)
}

// checkContainerVersion guards the day-one failure mode of an outdated CLI:
// vessel's backend speaks the JSON shapes probed on 1.2.x, so an older CLI
// must be named loudly instead of failing later with confusing parse errors.
func checkContainerVersion(r *report, path string) {
	out, err := runCmd(path, "--version")
	line := strings.TrimSpace(string(out))
	if err != nil {
		r.failf("container version: ERROR %v\n%s\n", err, indent(line))
		return
	}
	v, ok := parseSemver(line)
	if !ok {
		r.failf("container version: UNPARSEABLE %q (want at least %s)\n", line, minContainerVersion)
		return
	}
	if compareSemver(v, minContainerVersion) < 0 {
		r.failf("container version: %s BELOW MINIMUM %s (brew upgrade container)\n", v, minContainerVersion)
		return
	}
	r.linef("container version: %s\n", v)
}

func checkSystemStatus(r *report, path string) {
	out, err := runCmd(path, "system", "status")
	if err != nil {
		r.failf("system status: ERROR %v\n%s\n", err, strings.TrimSpace(string(out)))
		return
	}
	r.linef("system status:\n%s\n", indent(string(out)))
}

// checkPlatform covers the other two day-one failure modes from README
// Requirements: a non-Apple-silicon host and a macOS older than the supported
// floor (15..25 still runs, but only with the documented limitations).
func checkPlatform(r *report) {
	if hostArch != supportedArch {
		r.failf("architecture: %s UNSUPPORTED (%s required)\n", hostArch, supportedArch)
		return
	}
	r.linef("architecture: %s\n", hostArch)

	out, err := runCmd(macOSVersionCmd, "-productVersion")
	line := strings.TrimSpace(string(out))
	if err != nil {
		r.failf("macOS version: UNKNOWN (%v)\n%s\n", err, indent(line))
		return
	}
	major, ok := majorVersion(line)
	if !ok {
		r.failf("macOS version: UNPARSEABLE %q (want macOS %d+)\n", line, requiredMacOSMajor)
		return
	}
	switch {
	case major >= requiredMacOSMajor:
		r.linef("macOS version: %s\n", line)
	case major >= warnMacOSMajor:
		r.linef("macOS version: %s LIMITED (macOS %d+ recommended)\n", line, requiredMacOSMajor)
	default:
		r.failf("macOS version: %s UNSUPPORTED (macOS %d+ required)\n", line, requiredMacOSMajor)
	}
}

func checkConfig(r *report) {
	cfgPath := config.Path()
	r.linef("config path: %s\n", cfgPath)
	cfg, err := config.Load()
	if err != nil {
		r.failf("config load: ERROR %v\n", err)
		return
	}
	r.linef("config: poll=%s log_tail=%d mouse=%v shell=%q\n",
		cfg.PollInterval.Duration, cfg.LogTailLines, cfg.MouseEnabled, cfg.Shell)
}

// A missing pbcopy stays non-fatal on purpose: it degrades one keybinding,
// not the dashboard itself.
func checkClipboard(r *report) {
	if _, err := lookPath(clipboardBinary); err != nil {
		r.linef("pbcopy: MISSING (yank will fail)\n")
		return
	}
	r.linef("pbcopy: ok\n")
}

// semver is a dotted version triple; absent components count as zero, so
// "1.2" parses to the same value as "1.2.0".
type semver [3]int

func (v semver) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

// compareSemver orders two triples lexicographically: negative when a < b.
func compareSemver(a, b semver) int {
	for i := range a {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}
	return 0
}

// versionPattern lifts the first dotted numeric token out of CLI banner text
// like "container CLI version 1.2.2 (build: release, commit: unspeci)". The
// dot is required, so bare build numbers never match by accident.
var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

func parseSemver(s string) (semver, bool) {
	var v semver
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return v, false
	}
	for i, part := range m[1:] {
		if part == "" {
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}

// majorVersion reads the leading number of an OS product version like
// "26.5.2"; it is the piece README's macOS floor is written against.
func majorVersion(productVersion string) (int, bool) {
	v, ok := parseSemver(productVersion)
	return v[0], ok
}

func indent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
