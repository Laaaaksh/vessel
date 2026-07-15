package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Laaaaksh/vessel/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.New())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "vessel: %v\n", err)
		os.Exit(1)
	}
}
