package command

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewJobLeaveCmd allows a worker to leave a job.
func NewJobLeaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave a job as a worker",
		Long: `Leave the current job as a worker.

Uses FRAY_AGENT_ID to identify the worker. The worker's presence is set to
offline but the agent record remains (ephemeral agents persist).

If --as is provided, uses that worker ID instead of FRAY_AGENT_ID.

Examples:
  fray job leave                    # Uses FRAY_AGENT_ID
  fray job leave --as dev[abc1-0]   # Explicit worker ID`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			asRef, _ := cmd.Flags().GetString("as")
			workerID := asRef
			if workerID == "" {
				workerID = os.Getenv("FRAY_AGENT_ID")
			}
			if workerID == "" {
				return writeCommandError(cmd, fmt.Errorf("no worker ID: set FRAY_AGENT_ID or use --as"))
			}
			workerID = core.NormalizeAgentRef(workerID)

			// Parse worker ID to extract job info
			baseAgent, _, _, isWorker := core.ParseJobWorkerName(workerID)
			if !isWorker {
				return writeCommandError(cmd, fmt.Errorf("%s is not a job worker ID (expected format: agent[suffix-idx])", workerID))
			}

			// Get worker agent
			worker, err := db.GetAgent(ctx.DB, workerID)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("failed to look up worker: %w", err))
			}
			if worker == nil {
				return writeCommandError(cmd, fmt.Errorf("worker not found: %s", workerID))
			}
			if worker.JobID == nil {
				return writeCommandError(cmd, fmt.Errorf("%s is not associated with a job", workerID))
			}

			jobID := *worker.JobID
			now := time.Now().Unix()

			// Update worker presence to offline
			if err := db.UpdateAgentPresence(ctx.DB, workerID, types.PresenceOffline); err != nil {
				return writeCommandError(cmd, err)
			}

			// Append leave event to JSONL
			leaveRecord := db.JobWorkerLeaveJSONLRecord{
				Type:     "job_worker_leave",
				JobGUID:  jobID,
				AgentID:  baseAgent,
				WorkerID: workerID,
				LeftAt:   now,
			}
			if err := db.AppendJobWorkerLeave(ctx.Project.DBPath, leaveRecord); err != nil {
				return writeCommandError(cmd, err)
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"worker_id": workerID,
					"job_id":    jobID,
					"left_at":   now,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Left job %s as %s\n", jobID, workerID)
			return nil
		},
	}

	cmd.Flags().String("as", "", "worker ID (default: FRAY_AGENT_ID)")

	return cmd
}
