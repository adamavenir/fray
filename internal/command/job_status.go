package command

import (
	"encoding/json"
	"fmt"

	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// JobStatusOutput is the JSON output for job status.
type JobStatusOutput struct {
	GUID        string             `json:"guid"`
	Name        string             `json:"name"`
	Status      types.JobStatus    `json:"status"`
	OwnerAgent  string             `json:"owner_agent"`
	ThreadGUID  string             `json:"thread_guid"`
	Context     *types.JobContext  `json:"context,omitempty"`
	CreatedAt   int64              `json:"created_at"`
	CompletedAt *int64             `json:"completed_at,omitempty"`
	Workers     []WorkerStatusInfo `json:"workers"`
}

// WorkerStatusInfo shows worker details.
type WorkerStatusInfo struct {
	WorkerID  string              `json:"worker_id"`
	JobIdx    int                 `json:"job_idx"`
	Presence  types.PresenceState `json:"presence"`
	LastSeen  int64               `json:"last_seen"`
}

// NewJobStatusCmd shows job status and workers.
func NewJobStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show job status and workers",
		Long: `Show detailed status for a job including all workers.

Examples:
  fray job status job-abc12345
  fray job status job-abc12345 --json`,
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

			job, err := db.GetJob(ctx.DB, jobID)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("failed to look up job: %w", err))
			}
			if job == nil {
				return writeCommandError(cmd, fmt.Errorf("job not found: %s", jobID))
			}

			workers, err := db.GetJobWorkers(ctx.DB, jobID)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("failed to get workers: %w", err))
			}

			if ctx.JSONMode {
				var workerInfos []WorkerStatusInfo
				for _, w := range workers {
					idx := 0
					if w.JobIdx != nil {
						idx = *w.JobIdx
					}
					workerInfos = append(workerInfos, WorkerStatusInfo{
						WorkerID:  w.AgentID,
						JobIdx:    idx,
						Presence:  w.Presence,
						LastSeen:  w.LastSeen,
					})
				}
				output := JobStatusOutput{
					GUID:        job.GUID,
					Name:        job.Name,
					Status:      job.Status,
					OwnerAgent:  job.OwnerAgent,
					ThreadGUID:  job.ThreadGUID,
					Context:     job.Context,
					CreatedAt:   job.CreatedAt,
					CompletedAt: job.CompletedAt,
					Workers:     workerInfos,
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
			}

			// Text output
			fmt.Fprintf(cmd.OutOrStdout(), "Job: %s\n", job.GUID)
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", job.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", job.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "Owner: @%s\n", job.OwnerAgent)
			fmt.Fprintf(cmd.OutOrStdout(), "Thread: %s\n", job.ThreadGUID)

			if len(workers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Workers: (none)")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Workers:")
				for _, w := range workers {
					presence := string(w.Presence)
					if presence == "" {
						presence = "offline"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  @%s [%s]\n", w.AgentID, presence)
				}
			}

			return nil
		},
	}

	return cmd
}
