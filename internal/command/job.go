package command

import (
	"github.com/spf13/cobra"
)

// NewJobCmd creates the parent job command.
func NewJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage parallel work jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		NewJobCreateCmd(),
		NewJobJoinCmd(),
		NewJobCloseCmd(),
		NewJobLeaveCmd(),
		NewJobListCmd(),
		NewJobStatusCmd(),
	)

	return cmd
}
