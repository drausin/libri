package cmd

import (
	"github.com/spf13/cobra"
	"github.com/pkg/errors"
)

var errFailedHealthcheck = errors.New("some or all librarians unhealthy")

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "check health of librarian peers",
	Long:  `TODO (drausin) more detailed description`,
	RunE: func(cmd *cobra.Command, args []string) error {
		author, _, err := newTestAuthorGetter().get()
		if err != nil {
			return err
		}
		if allHealthy, _ := author.Healthcheck(); !allHealthy {
			return errFailedHealthcheck
		}
		return nil
	},
}

func init() {
	testCmd.AddCommand(healthCmd)
}
