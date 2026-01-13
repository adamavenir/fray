package command

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewJobCreateCmd creates a job and its associated thread.
func NewJobCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a parallel work job",
		Long: `Create a new job for parallel work coordination.

A job represents a unit of parallel work that can be distributed across
multiple worker agents. Creating a job:
- Generates a unique job GUID
- Creates an associated thread for coordination
- Records optional context (issues, threads, messages)

Examples:
  fray job create "implement auth" --as pm
  fray job create "refactor db" --context '{"issues":["fray-abc"]}' --as pm
  fray job create "test suite" --as pm --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			jobName := args[0]
			asRef, _ := cmd.Flags().GetString("as")
			contextJSON, _ := cmd.Flags().GetString("context")

			// Resolve owner agent
			if asRef == "" {
				return writeCommandError(cmd, fmt.Errorf("--as <agent> required"))
			}
			ownerAgent, err := resolveAgentRef(ctx, asRef)
			if err != nil {
				return writeCommandError(cmd, err)
			}

			// Parse context if provided
			var jobContext *types.JobContext
			if contextJSON != "" {
				jobContext = &types.JobContext{}
				if err := json.Unmarshal([]byte(contextJSON), jobContext); err != nil {
					return writeCommandError(cmd, fmt.Errorf("invalid context JSON: %w", err))
				}
			}

			// Generate job GUID
			jobGUID, err := core.GenerateGUID("job")
			if err != nil {
				return writeCommandError(cmd, err)
			}

			// Generate thread GUID (thread name = job guid for easy association)
			threadGUID, err := core.GenerateGUID("thrd")
			if err != nil {
				return writeCommandError(cmd, err)
			}

			now := time.Now().Unix()

			// Create the thread first
			thread := types.Thread{
				GUID:           threadGUID,
				Name:           jobGUID, // thread name is the job GUID
				Status:         types.ThreadStatusOpen,
				Type:           types.ThreadTypeStandard,
				CreatedAt:      now,
				CreatedBy:      &ownerAgent,
				LastActivityAt: &now,
			}

			if _, err := db.CreateThread(ctx.DB, thread); err != nil {
				return writeCommandError(cmd, err)
			}

			// Append thread to JSONL
			if err := db.AppendThread(ctx.Project.DBPath, thread, nil); err != nil {
				return writeCommandError(cmd, err)
			}

			// Create the job
			job := types.Job{
				GUID:       jobGUID,
				Name:       jobName,
				Context:    jobContext,
				OwnerAgent: ownerAgent,
				Status:     types.JobStatusRunning,
				ThreadGUID: threadGUID,
				CreatedAt:  now,
			}

			if err := db.CreateJob(ctx.DB, job); err != nil {
				return writeCommandError(cmd, err)
			}

			// Append job create to JSONL
			if err := db.AppendJobCreate(ctx.Project.DBPath, job); err != nil {
				return writeCommandError(cmd, err)
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"guid":        job.GUID,
					"name":        job.Name,
					"owner_agent": job.OwnerAgent,
					"status":      job.Status,
					"thread_guid": job.ThreadGUID,
					"context":     job.Context,
					"created_at":  job.CreatedAt,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", job.GUID)
			return nil
		},
	}

	cmd.Flags().String("as", "", "owner agent (required)")
	cmd.Flags().String("context", "", "job context as JSON")
	_ = cmd.MarkFlagRequired("as")

	return cmd
}
