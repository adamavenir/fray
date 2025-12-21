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

	cmd.PersistentFlags().String("project", "", "operate in linked project")
	cmd.PersistentFlags().String("in", "", "operate in channel context")
	cmd.PersistentFlags().Bool("json", false, "output in JSON format")

	cmd.AddCommand(
		NewInitCmd(),
		NewNewCmd(),
		NewPostCmd(),
		NewGetCmd(),
	)

	return cmd
}

func Execute() error {
	return NewRootCmd(Version).Execute()
}
