package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "libri",
	Short: "libri is a public peer-to-peer encrypted data storage network",
	Long:  `TODO (drausin) add longer description & examples here`,
}

// Execute is the main entrypoint for the libri CLI.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.libri.yml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
}
