package cli

import "github.com/spf13/cobra"

type rootOptions struct {
	configFile string
}

// NewRootCommand creates the Prompt Gate backend command tree.
func NewRootCommand() *cobra.Command {
	options := rootOptions{}
	root := &cobra.Command{
		Use:           "promptgate",
		Short:         "Prompt Gate backend service",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&options.configFile, "config", "", "Path to the Prompt Gate YAML configuration file")
	_ = root.MarkPersistentFlagRequired("config")

	root.AddCommand(
		newAPICommand(&options.configFile),
		newProxyCommand(&options.configFile),
		newMigrateCommand(&options.configFile),
		newScheduleCommand(&options.configFile),
		newWorkerCommand(&options.configFile),
	)
	return root
}
