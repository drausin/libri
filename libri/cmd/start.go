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
	"net"
)

const (
	localIPFlag    = "localIP"
	localPortFlag  = "localPort"
	publicIPFlag   = "publicIP"
	publicPortFlag = "publicPort"
	publicNameFlag = "publicName"
	dataDirFlag    = "dataDir"
	dbDirFlag      = "dbDir"
	bootstrapsFlag = "bootstraps"
	logLevelFlag   = "logLevel"
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

	startLibrarianCmd.Flags().String(localIPFlag, server.DefaultIP,
		"local IPv4 address")
	startLibrarianCmd.Flags().Int(localPortFlag, server.DefaultPort,
		"local port")
	startLibrarianCmd.Flags().StringP(publicIPFlag, "i", server.DefaultIP,
		"public IPv4 address")
	startLibrarianCmd.Flags().IntP(publicPortFlag, "p", server.DefaultPort,
		"public port")
	startLibrarianCmd.Flags().StringP(publicNameFlag, "n", "",
		"public peer name")
	startLibrarianCmd.Flags().StringP(dataDirFlag, "d", "",
		"local data directory")
	startLibrarianCmd.Flags().String(dbDirFlag, "",
		"local DB directory")
	startLibrarianCmd.Flags().StringArrayP(bootstrapsFlag, "b", nil,
		"comma-separated addresses (IPv4:Port) of bootstrap peers")
	startLibrarianCmd.Flags().StringP(logLevelFlag, "l", zap.InfoLevel.String(),
		"log level")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI") // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()        // read in environment variables that match
	if err := viper.BindPFlags(startLibrarianCmd.Flags()); err != nil {
		panic(err)
	}
}

func getLibrarianConfig() (*server.Config, *zap.Logger, error) {
	localAddr, err := server.ParseAddr(
		viper.GetString(localIPFlag),
		viper.GetInt(localPortFlag),
	)
	if err != nil {
		log.Printf("fatal error parsing local address: %v", err)
		return nil, nil, err
	}
	publicAddr, err := server.ParseAddr(
		viper.GetString(publicIPFlag),
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
		WithDBDir(viper.GetString(dbDirFlag)).
		WithLogLevel(getLogLevel())

	logger := clogging.NewDevLogger(config.LogLevel)
	addr, err := net.ResolveTCPAddr("tcp4", viper.GetStringSlice(bootstrapsFlag)[0])
	log.Printf("addr: %v, err: %v", addr, err)
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
		zap.String(dbDirFlag, config.DbDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
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
