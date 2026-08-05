package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/Laaaaksh/vessel/internal/doctor"
	"github.com/Laaaaksh/vessel/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "doctor":
			os.Exit(doctor.Run())
		case "help", "-h", "--help":
			fmt.Fprintf(os.Stdout, "usage: vessel [doctor]\n")
			os.Exit(0)
		}
	}

	p := tea.NewProgram(ui.New())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vessel: %v\n", err)
		os.Exit(1)
	}
}
