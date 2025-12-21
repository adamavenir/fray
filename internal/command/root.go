package command

import (
	"os"

	"github.com/spf13/cobra"
)

const AppName = "mm"

// Version is overwritten at build time using -ldflags.
var Version = "dev"

func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           AppName,
		Short:         "Mini Messenger - CLI for agent-to-agent messaging",
		Long:          "Mini Messenger (mm) is a lightweight agent-to-agent messaging CLI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.Version = version
	cmd.SetVersionTemplate(AppName + " version {{.Version}}\n")
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	return cmd
}

func Execute() error {
	return NewRootCmd(Version).Execute()
}
