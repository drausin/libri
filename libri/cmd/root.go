package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"github.com/drausin/libri/libri/common/errors"
)

const (
	dataDirFlag  = "dataDir"
	logLevelFlag = "logLevel"
	envVarPrefix = "LIBRI"
)

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
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringP(dataDirFlag, "d", "",
		"local data directory")
	RootCmd.PersistentFlags().StringP(logLevelFlag, "l", zap.InfoLevel.String(),
		"log level")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI") // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()        // read in environment variables that match
	errors.MaybePanic(viper.BindPFlags(RootCmd.PersistentFlags()))
}

func getLogLevel() zapcore.Level {
	var ll zapcore.Level
	errors.MaybePanic(ll.Set(viper.GetString(logLevelFlag)))
	return ll
}
