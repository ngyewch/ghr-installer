package cmd

import (
	"fmt"
	versionInfoCobra "github.com/ngyewch/go-versioninfo/cobra"
	"github.com/spf13/cobra"
	"os"
)

var (
	rootCmd = &cobra.Command{
		Use:   fmt.Sprintf("%s [flags]", appName),
		Short: "GitHub Release installer",
		RunE:  help,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func help(cmd *cobra.Command, args []string) error {
	err := cmd.Help()
	if err != nil {
		return err
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	versionInfoCobra.AddVersionCmd(rootCmd, nil)
}

func initConfig() {
}
