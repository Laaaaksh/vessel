package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/Laaaaksh/vessel/internal/doctor"
	"github.com/Laaaaksh/vessel/internal/ui"
)

// version, commit, and date are set at build time via
// -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
// They must remain package-level vars: the linker's -X flag can only
// overwrite a var of type string, not a const.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// usage is printed by help and by every unrecognized argument, so a typo'd
// subcommand fails loudly instead of silently opening the dashboard.
const usage = `usage: vessel [command]

commands:
  vessel              start the dashboard
  vessel doctor       check CLI, system status, config
  vessel version      print version information
  vessel help         show this message`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "doctor":
			os.Exit(doctor.Run())
		case "version", "-v", "--version":
			fmt.Printf("vessel version %s (commit %s, built %s)\n", version, commit, date)
			os.Exit(0)
		case "help", "-h", "--help":
			fmt.Println(usage)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "vessel: unknown command %q\n%s\n", os.Args[1], usage)
			os.Exit(2)
		}
	}

	p := tea.NewProgram(ui.New())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vessel: %v\n", err)
		os.Exit(1)
	}
}
