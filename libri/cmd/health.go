package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "check health of librarian peers",
	Long:  `TODO (drausin) more detailed description`,
	Run: func(cmd *cobra.Command, args []string) {
		author, logger, err := newTestAuthorGetter().get()
		if err != nil {
			logger.Error("fatal error while initializing author", zap.Error(err))
			os.Exit(1)
		}
		if allHealthy, _ := author.Healthcheck(); !allHealthy {
			os.Exit(1)
		}
	},
}

func init() {
	testCmd.AddCommand(healthCmd)
}
