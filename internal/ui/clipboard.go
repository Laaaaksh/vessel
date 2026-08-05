package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

// CopyToClipboard writes text to the macOS pasteboard via pbcopy.
func CopyToClipboard(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("nothing to copy")
	}
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}
	return nil
}
