package command

import (
	"encoding/json"
	"fmt"

	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/spf13/cobra"
)

// NewJobListCmd lists jobs.
func NewJobListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		Long: `List jobs with their status.

By default shows only running jobs. Use --all to include completed/cancelled.

Examples:
  fray job list              # Running jobs only
  fray job list --all        # All jobs
  fray job list --json       # JSON output`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			showAll, _ := cmd.Flags().GetBool("all")

			var jobs []types.Job
			if showAll {
				jobs, err = db.GetAllJobs(ctx.DB)
			} else {
				jobs, err = db.GetActiveJobs(ctx.DB)
			}
			if err != nil {
				return writeCommandError(cmd, err)
			}

			if ctx.JSONMode {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(jobs)
			}

			if len(jobs) == 0 {
				if showAll {
					fmt.Fprintln(cmd.OutOrStdout(), "No jobs found")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No running jobs")
				}
				return nil
			}

			for _, job := range jobs {
				workers, _ := db.GetJobWorkers(ctx.DB, job.GUID)
				workerCount := len(workers)
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s [%s] (%d workers)\n",
					job.GUID, job.Name, job.Status, workerCount)
			}

			return nil
		},
	}

	cmd.Flags().Bool("all", false, "show all jobs (including completed)")

	return cmd
}
