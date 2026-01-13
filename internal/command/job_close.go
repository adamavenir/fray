package command

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewJobCloseCmd closes a job with a specified status.
func NewJobCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <job-id>",
		Short: "Close a job",
		Long: `Close a job with a specified status.

By default sets status to 'completed'. Use --status to set a different status
(completed, cancelled, failed).

Workers remain registered but the job is no longer considered active.

Examples:
  fray job close job-abc12345                    # Default: completed
  fray job close job-abc12345 --status cancelled
  fray job close job-abc12345 --status failed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			jobID := args[0]
			if len(jobID) < 8 || jobID[:4] != "job-" {
				return writeCommandError(cmd, fmt.Errorf("invalid job ID: %s (must be job-<id>)", jobID))
			}

			statusStr, _ := cmd.Flags().GetString("status")
			var status types.JobStatus
			switch statusStr {
			case "", "completed":
				status = types.JobStatusCompleted
			case "cancelled":
				status = types.JobStatusCancelled
			case "failed":
				status = types.JobStatusFailed
			default:
				return writeCommandError(cmd, fmt.Errorf("invalid status: %s (use completed, cancelled, or failed)", statusStr))
			}

			// Get job
			job, err := db.GetJob(ctx.DB, jobID)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("failed to look up job: %w", err))
			}
			if job == nil {
				return writeCommandError(cmd, fmt.Errorf("job not found: %s", jobID))
			}
			if job.Status != types.JobStatusRunning {
				return writeCommandError(cmd, fmt.Errorf("job %s is already %s", jobID, job.Status))
			}

			now := time.Now().Unix()

			// Update in DB
			if err := db.UpdateJobStatus(ctx.DB, jobID, status, &now); err != nil {
				return writeCommandError(cmd, err)
			}

			// Append to JSONL
			if err := db.AppendJobUpdate(ctx.Project.DBPath, db.JobUpdateJSONLRecord{
				Type:        "job_update",
				GUID:        jobID,
				Status:      string(status),
				CompletedAt: &now,
			}); err != nil {
				return writeCommandError(cmd, err)
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"guid":      jobID,
					"status":    status,
					"closed_at": now,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Job %s closed (%s)\n", jobID, status)
			return nil
		},
	}

	cmd.Flags().String("status", "completed", "close status (completed, cancelled, failed)")

	return cmd
}
