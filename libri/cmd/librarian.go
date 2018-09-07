package cmd

import (
	"github.com/spf13/cobra"
)

// librarianCmd represents the librarian command
var librarianCmd = &cobra.Command{
	Use:   "librarian",
	Short: "operate a librarian server, a peer in the Libri network",
}

func init() {
	RootCmd.AddCommand(librarianCmd)
}
