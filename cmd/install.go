package cmd

import (
	"github.com/google/go-github/v56/github"
	"github.com/ngyewch/ghr-installer/installer"
	"github.com/spf13/cobra"
	"os"
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

	githubClient := github.NewClient(nil)
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken != "" {
		githubClient = githubClient.WithAuthToken(githubToken)
	}
	installer1 := installer.NewInstaller(baseDirectory, githubClient)

	err = installer1.Install(packageSpec)
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
