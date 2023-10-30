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
	baseDirectory, err := cmd.Flags().GetString("base-directory")
	if err != nil {
		return err
	}

	packageSpec := args[0]

	err = installer.Install(baseDirectory, packageSpec)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().String("base-directory", "", "base directory (REQUIRED)")

	err := installCmd.MarkFlagRequired("base-directory")
	if err != nil {
		panic(err)
	}
}
