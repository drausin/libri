package cmd

import (
	"os"

	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the librarian command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the libri version",
	Long:  `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		_, err := os.Stdout.WriteString(version.Version.String() + "\n")
		errors.MaybePanic(err)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
