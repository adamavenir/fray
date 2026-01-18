package hooks

import (
	"os"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewHookSessionEndCmd handles Claude SessionEnd hooks.
// This hook fires when a Claude Code session terminates, allowing fray to
// immediately update agent presence instead of waiting for process exit detection.
func NewHookSessionEndCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-sessionend [reason]",
		Short: "SessionEnd hook handler (internal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			output := hookOutput{}

			agentID := os.Getenv("FRAY_AGENT_ID")
			if agentID == "" {
				return writeHookOutput(cmd, output)
			}

			sessionID := os.Getenv("CLAUDE_SESSION_ID")
			reason := "exit"
			if len(args) > 0 {
				reason = args[0]
			}

			projectPath := os.Getenv("CLAUDE_PROJECT_DIR")
			project, err := core.DiscoverProject(projectPath)
			if err != nil {
				return writeHookOutput(cmd, output)
			}

			dbConn, err := db.OpenDatabase(project)
			if err != nil {
				return writeHookOutput(cmd, output)
			}
			defer dbConn.Close()
			if err := db.InitSchema(dbConn); err != nil {
				return writeHookOutput(cmd, output)
			}

			agent, err := db.GetAgent(dbConn, agentID)
			if err != nil || agent == nil {
				return writeHookOutput(cmd, output)
			}

			now := time.Now().Unix()

			// Record session end event
			sessionEnd := types.SessionEnd{
				AgentID:   agentID,
				SessionID: sessionID,
				ExitCode:  0,
				EndedAt:   now,
			}
			db.AppendSessionEnd(project.DBPath, sessionEnd)

			// Update presence to idle (unless already offline from fray bye)
			if agent.Presence != types.PresenceOffline {
				db.UpdateAgentPresenceWithAudit(
					dbConn, project.DBPath, agentID,
					agent.Presence, types.PresenceIdle,
					"sessionend_hook_"+reason, "hook",
					agent.Status,
				)
			}

			// Note: left_at should ONLY be set by fray bye, not when sessions naturally end.
			// Sessions that end without fray bye are resumable via @mention (presence=idle).
			// Sessions that end with fray bye are explicitly offline (presence=offline, left_at set).

			return writeHookOutput(cmd, output)
		},
	}

	return cmd
}
