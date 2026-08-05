package doctor

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Laaaaksh/vessel/internal/config"
)

// Run prints environment diagnostics to stdout and returns a process exit code.
func Run() int {
	ok := true
	printf := func(format string, args ...any) { fmt.Printf(format, args...) }

	printf("vessel doctor\n")
	printf("-------------\n")

	path, err := exec.LookPath("container")
	if err != nil {
		printf("container CLI: MISSING (%v)\n", err)
		ok = false
	} else {
		printf("container CLI: %s\n", path)
		out, err := exec.Command(path, "system", "status").CombinedOutput()
		if err != nil {
			printf("system status: ERROR %v\n%s\n", err, strings.TrimSpace(string(out)))
			ok = false
		} else {
			printf("system status:\n%s\n", indent(string(out)))
		}
	}

	cfgPath := config.ConfigPath()
	printf("config path: %s\n", cfgPath)
	cfg, err := config.Load()
	if err != nil {
		printf("config load: ERROR %v\n", err)
		ok = false
	} else {
		printf("config: poll=%s log_tail=%d mouse=%v shell=%q\n",
			cfg.PollInterval.Duration, cfg.LogTailLines, cfg.MouseEnabled, cfg.Shell)
	}

	if _, err := exec.LookPath("pbcopy"); err != nil {
		printf("pbcopy: MISSING (yank will fail)\n")
	} else {
		printf("pbcopy: ok\n")
	}

	if ok {
		printf("\nok\n")
		return 0
	}
	printf("\nproblems detected\n")
	return 1
}

func indent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
