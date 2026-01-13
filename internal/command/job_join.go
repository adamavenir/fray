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

// NewJobJoinCmd registers a worker for a job.
func NewJobJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "join <job-id>",
		Short: "Register as a worker for a job",
		Long: `Register as a worker for a parallel job session.

The worker ID is constructed as <agent>[<4-char-suffix>-<idx>], where:
- <agent> is the base agent name (from --as)
- <4-char-suffix> is the first 4 characters after "job-" prefix
- <idx> is the worker index (auto-assigned or from --idx)

Examples:
  fray job join job-abc12345 --as dev           # Gets next available index
  fray job join job-abc12345 --as dev --idx 0   # Explicit index

The worker is registered as an ephemeral agent with job context.
If CLAUDE_ENV_FILE is set, writes FRAY_AGENT_ID, FRAY_JOB_ID, FRAY_JOB_IDX.`,
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

			agentID, _ := cmd.Flags().GetString("as")
			if agentID == "" {
				return writeCommandError(cmd, fmt.Errorf("--as is required"))
			}
			agentID = core.NormalizeAgentRef(agentID)

			// Validate job exists and is running
			job, err := db.GetJob(ctx.DB, jobID)
			if err != nil {
				return writeCommandError(cmd, fmt.Errorf("failed to look up job: %w", err))
			}
			if job == nil {
				return writeCommandError(cmd, fmt.Errorf("job not found: %s", jobID))
			}
			if job.Status != types.JobStatusRunning {
				return writeCommandError(cmd, fmt.Errorf("job %s is %s, not running", jobID, job.Status))
			}

			// Get explicit idx or auto-assign
			idxFlag, _ := cmd.Flags().GetInt("idx")
			var jobIdx int
			if cmd.Flags().Changed("idx") {
				jobIdx = idxFlag
			} else {
				// Auto-assign next available index from existing workers
				workers, err := db.GetJobWorkers(ctx.DB, jobID)
				if err != nil {
					return writeCommandError(cmd, fmt.Errorf("failed to get job workers: %w", err))
				}
				jobIdx = len(workers)
			}

			// Construct worker ID: <agent>[<4-char-suffix>-<idx>]
			// job-abc12345 → suffix "abc1", workerID "dev[abc1-0]"
			suffix := jobID[4:8] // First 4 chars after "job-"
			workerID := fmt.Sprintf("%s[%s-%d]", agentID, suffix, jobIdx)

			// Create ephemeral agent for worker
			now := time.Now().Unix()
			agentGUID, err := core.GenerateGUID("usr")
			if err != nil {
				return writeCommandError(cmd, err)
			}

			worker := types.Agent{
				GUID:         agentGUID,
				AgentID:      workerID,
				RegisteredAt: now,
				LastSeen:     now,
				Presence:     types.PresenceActive,
				JobID:        &jobID,
				JobIdx:       &jobIdx,
				IsEphemeral:  true,
			}

			// TODO: Check if worker exists and is offline for upsert semantics
			if err := db.CreateAgent(ctx.DB, worker); err != nil {
				return writeCommandError(cmd, err)
			}
			if err := db.AppendAgent(ctx.Project.DBPath, worker); err != nil {
				return writeCommandError(cmd, err)
			}

			// Append job_worker_join event to JSONL
			joinRecord := db.JobWorkerJoinJSONLRecord{
				Type:     "job_worker_join",
				JobGUID:  jobID,
				AgentID:  agentID,
				WorkerID: workerID,
				JobIdx:   jobIdx,
				JoinedAt: now,
			}
			if err := db.AppendJobWorkerJoin(ctx.Project.DBPath, joinRecord); err != nil {
				return writeCommandError(cmd, err)
			}

			// Write env vars if CLAUDE_ENV_FILE is set
			wroteEnv := writeJobEnv(workerID, jobID, jobIdx)

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"worker_id":  workerID,
					"job_id":     jobID,
					"job_idx":    jobIdx,
					"agent_id":   agentID,
					"claude_env": wroteEnv,
				})
			}

			fmt.Fprintln(cmd.OutOrStdout(), workerID)
			return nil
		},
	}

	cmd.Flags().String("as", "", "base agent name (required)")
	cmd.Flags().Int("idx", 0, "worker index (auto-assigned if not specified)")
	cmd.MarkFlagRequired("as")

	return cmd
}

// writeJobEnv writes job-related env vars to CLAUDE_ENV_FILE.
func writeJobEnv(workerID, jobID string, jobIdx int) bool {
	path := os.Getenv("CLAUDE_ENV_FILE")
	if path == "" {
		return false
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false
	}
	defer file.Close()

	lines := fmt.Sprintf("FRAY_AGENT_ID=%s\nFRAY_JOB_ID=%s\nFRAY_JOB_IDX=%d\n", workerID, jobID, jobIdx)
	if _, err := file.WriteString(lines); err != nil {
		return false
	}
	return true
}
