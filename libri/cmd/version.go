package cmd

import (
	"github.com/spf13/cobra"
	"github.com/drausin/libri/version"
	"os"
)

// versionCmd represents the librarian command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the libri version",
	Long:  `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		os.Stdout.WriteString(version.Version.String() + "\n")
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
