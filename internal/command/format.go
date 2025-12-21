package command

import (
	"fmt"
	"path/filepath"

	"github.com/adamavenir/mini-msg/internal/types"
)

// GetProjectName returns the basename for the project root.
func GetProjectName(projectRoot string) string {
	return filepath.Base(projectRoot)
}

// FormatMessage formats a message for display.
func FormatMessage(msg types.Message, projectName string) string {
	return fmt.Sprintf("[#%s %s] @%s: %s", projectName, msg.ID, msg.FromAgent, msg.Body)
}
