package hooks

import (
	"fmt"
	"os"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewHookPrecompactCmd handles Claude PreCompact hooks.
func NewHookPrecompactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-precompact",
		Short: "PreCompact hook handler (internal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			output := hookOutput{}

			agentID := os.Getenv("FRAY_AGENT_ID")
			if agentID == "" {
				agentID = "<you>"
			}

			// Send heartbeat to keep agent active through compaction.
			// This resets error state to active if needed.
			if agentID != "<you>" {
				projectPath := os.Getenv("CLAUDE_PROJECT_DIR")
				if project, err := core.DiscoverProject(projectPath); err == nil {
					if dbConn, err := db.OpenDatabase(project); err == nil {
						defer dbConn.Close()
						if err := db.InitSchema(dbConn); err == nil {
							if agent, err := db.GetAgent(dbConn, agentID); err == nil && agent != nil {
								now := time.Now().UnixMilli()
								if agent.Presence == "error" {
									// Reset error state to active
									active := "active"
									_ = db.UpdateAgentPresence(dbConn, agentID, types.PresenceActive)
									_ = db.AppendAgentUpdate(project.DBPath, db.AgentUpdateJSONLRecord{
										AgentID:       agentID,
										Presence:      &active,
										LastHeartbeat: &now,
									})
								} else {
									// Just update heartbeat
									_ = db.UpdateAgentHeartbeat(dbConn, agentID, now)
									_ = db.AppendAgentUpdate(project.DBPath, db.AgentUpdateJSONLRecord{
										AgentID:       agentID,
										LastHeartbeat: &now,
									})
								}
							}
						}
					}
				}
			}

			output.AdditionalContext = buildPrecompactContext(agentID)
			return writeHookOutput(cmd, output)
		},
	}

	return cmd
}

func buildPrecompactContext(agentID string) string {
	return fmt.Sprintf(`[fray] Context compacting. Preserve your work:
1. fray post %s/notes "# Handoff ..." --as %s
2. bd close <completed-issues>
3. fray bye %s

Or run /land for full checklist.`, agentID, agentID, agentID)
}
