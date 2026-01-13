package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// PreToolInput is the JSON structure received from Claude Code PreToolUse hooks.
type PreToolInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// BashToolInput is the structure for Bash tool inputs.
type BashToolInput struct {
	Command string `json:"command"`
}

// NewHookPreToolCmd handles Claude PreToolUse hooks for fray commands.
func NewHookPreToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-pretool",
		Short: "PreToolUse hook handler (internal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var input PreToolInput
			if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing hook input: %v\n", err)
				os.Exit(1)
			}

			if input.ToolName != "Bash" {
				os.Exit(0)
			}

			var bashInput BashToolInput
			if err := json.Unmarshal(input.ToolInput, &bashInput); err != nil {
				os.Exit(0)
			}

			command := bashInput.Command
			if command == "" {
				os.Exit(0)
			}

			if !isFrayPostCommand(command) {
				os.Exit(0)
			}

			reminder := buildPostReminder(command)
			if reminder != "" {
				fmt.Fprint(os.Stderr, reminder)
				os.Exit(2)
			}

			os.Exit(0)
			return nil
		},
	}

	return cmd
}

var (
	frayPostRegex   = regexp.MustCompile(`^\s*fray\s+post\b`)
	replyToRegex    = regexp.MustCompile(`(?:--reply-to|-r)\s+\S+`)
	// Thread paths are: meta, roles, or paths containing slashes (e.g., meta/notes, user/notes)
	threadPathRegex = regexp.MustCompile(`^\s*fray\s+post\s+(?:meta|roles|[a-z][a-z0-9_-]*/[a-z][a-z0-9_/-]*)\s+`)
)

func isFrayPostCommand(command string) bool {
	return frayPostRegex.MatchString(command)
}

func buildPostReminder(command string) string {
	// Check for --force flag - skip all reminders
	if strings.Contains(command, "--force") {
		return ""
	}

	var reminders []string

	hasReplyTo := replyToRegex.MatchString(command)
	hasThreadPath := threadPathRegex.MatchString(command)
	isRoomPost := !hasThreadPath

	// Check trigger context from env vars (set by daemon or session hooks)
	triggerHome := os.Getenv("FRAY_TRIGGER_HOME")

	// Only remind about thread context when we have it and posting to room
	if triggerHome != "" && triggerHome != "room" && isRoomPost && !hasReplyTo {
		reminders = append(reminders, fmt.Sprintf("• Your session started from thread '%s' - consider posting there or using --reply-to", triggerHome))
	}

	if len(reminders) == 0 {
		return ""
	}

	return fmt.Sprintf("[fray post reminder]\n%s\n\nTo proceed anyway, add --force.\n", strings.Join(reminders, "\n"))
}
