package cmd

import (
	"fmt"
	"os"

	"github.com/drausin/libri/libri/common/errors"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	bootstrapsFlag       = "bootstraps"
	localHostFlag        = "localHost"
	localPortFlag        = "localPort"
	localMetricsPortFlag = "localMetricsPort"
	publicHostFlag       = "publicHost"
	publicNameFlag       = "publicName"
	publicPortFlag       = "publicPort"
	nSubscriptionsFlag   = "nSubscriptions"
	fpRateFlag           = "fpRate"

	logLocalAddr        = "localAddr"
	logLocalMetricsAddr = "localMetricsAddr"
	logPublicAddr       = "publicAddr"
)

// startLibrarianCmd represents the librarian start command
var startLibrarianCmd = &cobra.Command{
	Use:   "start",
	Short: "start a librarian server",
	Long:  `TODO (drausin) add longer description and examples here`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, logger, err := getLibrarianConfig()
		if err != nil {
			return err
		}
		up := make(chan *server.Librarian, 1)
		if err = server.Start(logger, config, up); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	librarianCmd.AddCommand(startLibrarianCmd)

	startLibrarianCmd.Flags().String(localHostFlag, server.DefaultIP,
		"local host (IPv4 or URL)")
	startLibrarianCmd.Flags().Int(localPortFlag, server.DefaultPort,
		"local port")
	startLibrarianCmd.Flags().Int(localMetricsPortFlag, server.DefaultMetricsPort,
		"local metrics port")
	startLibrarianCmd.Flags().StringP(publicHostFlag, "i", server.DefaultIP,
		"public host (IPv4 or URL)")
	startLibrarianCmd.Flags().IntP(publicPortFlag, "p", server.DefaultPort,
		"public port")
	startLibrarianCmd.Flags().StringSliceP(bootstrapsFlag, "b", nil,
		"comma-separated addresses (IPv4:Port) of bootstrap peers")
	startLibrarianCmd.Flags().StringP(publicNameFlag, "n", "",
		"public peer name")
	startLibrarianCmd.Flags().IntP(nSubscriptionsFlag, "s", subscribe.DefaultNSubscriptionsTo,
		"number of active subscriptions to other peers to maintain")
	startLibrarianCmd.Flags().Float32P(fpRateFlag, "f", subscribe.DefaultFPRate,
		"false positive rate for subscriptions to other peers")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI") // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()        // read in environment variables that match
	errors.MaybePanic(viper.BindPFlags(startLibrarianCmd.Flags()))
}

func getLibrarianConfig() (*server.Config, *zap.Logger, error) {
	logLevel := getLogLevel()
	logger := clogging.NewDevLogger(logLevel)

	localAddr, err := server.ParseAddr(
		viper.GetString(localHostFlag),
		viper.GetInt(localPortFlag),
	)
	if err != nil {
		logger.Error("fatal error parsing local address", zap.Error(err))
		return nil, nil, err
	}
	localMetricsAddr, err := server.ParseAddr(
		viper.GetString(localHostFlag),
		viper.GetInt(localMetricsPortFlag),
	)
	if err != nil {
		logger.Error("fatal error parsing local metrics address", zap.Error(err))
		return nil, nil, err
	}
	publicAddr, err := server.ParseAddr(
		viper.GetString(publicHostFlag),
		viper.GetInt(publicPortFlag),
	)
	if err != nil {
		logger.Error("fatal error parsing public address", zap.Error(err))
		return nil, nil, err
	}
	config := server.NewDefaultConfig().
		WithLocalAddr(localAddr).
		WithLocalMetricsAddr(localMetricsAddr).
		WithPublicAddr(publicAddr).
		WithPublicName(viper.GetString(publicNameFlag)).
		WithDataDir(viper.GetString(dataDirFlag)).
		WithDefaultDBDir(). // depends on DataDir
		WithLogLevel(logLevel)
	config.SubscribeTo.NSubscriptions = uint32(viper.GetInt(nSubscriptionsFlag))
	config.SubscribeTo.FPRate = float32(viper.GetFloat64(fpRateFlag))

	bootstrapNetAddrs, err := server.ParseAddrs(viper.GetStringSlice(bootstrapsFlag))
	if err != nil {
		logger.Error("unable to parse bootstrap peer address", zap.Error(err))
		return nil, nil, err

	}
	config.WithBootstrapAddrs(bootstrapNetAddrs)

	WriteLibrarianBanner(os.Stdout)
	logger.Info("librarian configuration",
		zap.Stringer(logLocalAddr, config.LocalAddr),
		zap.Stringer(logLocalMetricsAddr, config.LocalMetricsAddr),
		zap.Stringer(logPublicAddr, config.PublicAddr),
		zap.String(bootstrapsFlag, fmt.Sprintf("%v", config.BootstrapAddrs)),
		zap.String(publicNameFlag, config.PublicName),
		zap.String(dataDirFlag, config.DataDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
		zap.Uint32(nSubscriptionsFlag, config.SubscribeTo.NSubscriptions),
		zap.Float32(fpRateFlag, config.SubscribeTo.FPRate),
	)
	return config, logger, nil
}
