package cmd

import (
	"github.com/ngyewch/ghr-installer/installer"
	"github.com/spf13/cobra"
)

var (
	installCmd = &cobra.Command{
		Use:   "install (owner/project@version)",
		Short: "Install",
		Args:  cobra.ExactArgs(1),
		RunE:  install,
	}
)

func install(cmd *cobra.Command, args []string) error {
	packageSpec := args[0]

	err := installer.Install(packageSpec)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(installCmd)
}
