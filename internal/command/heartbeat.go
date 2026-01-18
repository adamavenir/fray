package command

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewHeartbeatCmd creates the heartbeat command for silent checkins.
func NewHeartbeatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heartbeat",
		Short: "Send a silent checkin to reset the done-detection timer",
		Long: `Send a heartbeat to prevent the daemon from recycling your session.

The daemon uses done-detection to recycle idle agent sessions. If you're doing
long-running work without posting to fray, send a heartbeat to reset the timer.

Any fray activity (posts, replies, threads) also resets the timer automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			agentID, _ := cmd.Flags().GetString("as")
			if agentID == "" {
				agentID = os.Getenv("FRAY_AGENT_ID")
			}
			if agentID == "" {
				return writeCommandError(cmd, fmt.Errorf("--as flag or FRAY_AGENT_ID env var required"))
			}

			agent, err := db.GetAgent(ctx.DB, agentID)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			if agent == nil {
				return writeCommandError(cmd, fmt.Errorf("agent not found: %s", agentID))
			}

			now := time.Now().UnixMilli()
			presenceReset := false

			// If agent is in error state, reset to active
			if agent.Presence == "error" {
				presenceReset = true
				active := "active"
				if err := db.UpdateAgentPresence(ctx.DB, agentID, types.PresenceActive); err != nil {
					return writeCommandError(cmd, err)
				}
				// Persist presence change to JSONL
				if err := db.AppendAgentUpdate(ctx.Project.DBPath, db.AgentUpdateJSONLRecord{
					AgentID:       agentID,
					Presence:      &active,
					LastHeartbeat: &now,
				}); err != nil {
					return writeCommandError(cmd, err)
				}
			} else {
				// Update heartbeat in SQLite
				if err := db.UpdateAgentHeartbeat(ctx.DB, agentID, now); err != nil {
					return writeCommandError(cmd, err)
				}

				// Persist to JSONL
				if err := db.AppendAgentUpdate(ctx.Project.DBPath, db.AgentUpdateJSONLRecord{
					AgentID:       agentID,
					LastHeartbeat: &now,
				}); err != nil {
					return writeCommandError(cmd, err)
				}
			}

			if ctx.JSONMode {
				result := map[string]any{
					"agent_id":       agentID,
					"heartbeat":      now,
					"presence_reset": presenceReset,
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}

			if presenceReset {
				fmt.Fprintf(cmd.OutOrStdout(), "Heartbeat sent for @%s (presence reset: error → active)\n", agentID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Heartbeat sent for @%s\n", agentID)
			}
			return nil
		},
	}

	cmd.Flags().String("as", "", "agent sending the heartbeat (uses FRAY_AGENT_ID if not set)")

	return cmd
}
