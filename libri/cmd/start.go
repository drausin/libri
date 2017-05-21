package cmd

import (
	"os"

	"fmt"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"github.com/drausin/libri/libri/common/subscribe"
)

const (
	bootstrapsFlag = "bootstraps"
	dataDirFlag    = "dataDir"
	localHostFlag  = "localHost"
	localPortFlag  = "localPort"
	logLevelFlag   = "logLevel"
	publicHostFlag = "publicHost"
	publicNameFlag = "publicName"
	publicPortFlag = "publicPort"
	nSubscriptionsFlag = "nSubscriptions"
)

// startLibrarianCmd represents the librarian start command
var startLibrarianCmd = &cobra.Command{
	Use:   "start",
	Short: "start a librarian server",
	Long:  `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		config, logger, err := getLibrarianConfig()
		if err != nil {
			os.Exit(1)
		}
		up := make(chan *server.Librarian, 1)
		if err = server.Start(logger, config, up); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	librarianCmd.AddCommand(startLibrarianCmd)

	startLibrarianCmd.Flags().String(localHostFlag, server.DefaultIP,
		"local host (IPv4 or URL)")
	startLibrarianCmd.Flags().Int(localPortFlag, server.DefaultPort,
		"local port")
	startLibrarianCmd.Flags().StringP(publicHostFlag, "i", server.DefaultIP,
		"public host (IPv4 or URL)")
	startLibrarianCmd.Flags().IntP(publicPortFlag, "p", server.DefaultPort,
		"public port")
	startLibrarianCmd.Flags().StringSliceP(bootstrapsFlag, "b", nil,
		"comma-separated addresses (IPv4:Port) of bootstrap peers")
	startLibrarianCmd.Flags().StringP(publicNameFlag, "n", "",
		"public peer name")
	startLibrarianCmd.Flags().StringP(dataDirFlag, "d", "",
		"local data directory")
	startLibrarianCmd.Flags().StringP(logLevelFlag, "l", zap.InfoLevel.String(),
		"log level")
	startLibrarianCmd.Flags().IntP(nSubscriptionsFlag, "s", subscribe.DefaultNSubscriptionsTo,
		"number of active subscriptions to other peers to maintain")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI") // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()        // read in environment variables that match
	if err := viper.BindPFlags(startLibrarianCmd.Flags()); err != nil {
		panic(err)
	}
}

func getLibrarianConfig() (*server.Config, *zap.Logger, error) {
	localAddr, err := server.ParseAddr(
		viper.GetString(localHostFlag),
		viper.GetInt(localPortFlag),
	)
	if err != nil {
		log.Printf("fatal error parsing local address: %v", err)
		return nil, nil, err
	}
	publicAddr, err := server.ParseAddr(
		viper.GetString(publicHostFlag),
		viper.GetInt(publicPortFlag),
	)
	if err != nil {
		log.Printf("fatal error parsing public address: %v", err)
		return nil, nil, err
	}
	config := server.NewDefaultConfig().
		WithLocalAddr(localAddr).
		WithPublicAddr(publicAddr).
		WithPublicName(viper.GetString(publicNameFlag)).
		WithDataDir(viper.GetString(dataDirFlag)).
		WithLogLevel(getLogLevel())
	config.SubscribeTo.NSubscriptions = uint32(viper.GetInt(nSubscriptionsFlag))

	logger := clogging.NewDevLogger(config.LogLevel)
	bootstrapNetAddrs, err := server.ParseAddrs(viper.GetStringSlice(bootstrapsFlag))
	if err != nil {
		logger.Error("unable to parse bootstrap peer address", zap.Error(err))
		return nil, logger, err

	}
	config.WithBootstrapAddrs(bootstrapNetAddrs)

	logger.Info("librarian configuration",
		zap.Stringer("localAddress", config.LocalAddr),
		zap.Stringer("publicAddress", config.PublicAddr),
		zap.String(bootstrapsFlag, fmt.Sprintf("%v", config.BootstrapAddrs)),
		zap.String(publicNameFlag, config.PublicName),
		zap.String(dataDirFlag, config.DataDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
		zap.Uint32(nSubscriptionsFlag, config.SubscribeTo.NSubscriptions),
	)
	return config, logger, nil
}

func getLogLevel() zapcore.Level {
	var ll zapcore.Level
	err := ll.Set(viper.GetString(logLevelFlag))
	if err != nil {
		panic(err)
	}
	return ll
}
