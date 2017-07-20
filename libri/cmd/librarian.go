package cmd

import (
	"github.com/spf13/cobra"
)

// librarianCmd represents the librarian command
var librarianCmd = &cobra.Command{
	Use:   "librarian",
	Short: "operate a librarian server, a peer in the libri network",
	Long:  `TODO (drausin) add longer description and examples here`,
}

type LibrarianBanner struct {
	Now string
	GoVersion string
	GoOS string
	GoArch string
	NumCPU int
	GoMaxProcs int
}

func init() {
	RootCmd.AddCommand(librarianCmd)
}


