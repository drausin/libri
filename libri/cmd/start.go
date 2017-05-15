package cmd

import (
	"os"

	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// flags set below
var (
	localIP        string
	localPort      int
	publicIP       string
	publicPort     int
	publicName     string
	dataDir        string
	dbDir          string
	bootstrapAddrs []string
	logLevel       string
)

// startLibrarianCmd represents the librarian start command
var startLibrarianCmd = &cobra.Command{
	Use:   "start",
	Short: "start a librarian server",
	Long:  `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		config, logger, err := getConfig()
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

	startLibrarianCmd.Flags().StringVarP(&localIP, "local-ip", "i", server.DefaultIP,
		"local IPv4 address")
	startLibrarianCmd.Flags().IntVarP(&localPort, "local-port", "p", server.DefaultPort,
		"local port")
	startLibrarianCmd.Flags().StringVarP(&publicIP, "public-ip", "j", server.DefaultIP,
		"public IPv4 address")
	startLibrarianCmd.Flags().IntVarP(&publicPort, "public-port", "q", server.DefaultPort,
		"public port")
	startLibrarianCmd.Flags().StringVarP(&publicName, "public-name", "n", "",
		"public peer name")
	startLibrarianCmd.Flags().StringVarP(&dataDir, "data-dir", "d", "",
		"local data directory")
	startLibrarianCmd.Flags().StringVarP(&dbDir, "db-dir", "b", "",
		"local DB directory")
	startLibrarianCmd.Flags().StringArrayVarP(&bootstrapAddrs, "boostrap-addrs", "a", nil,
		"comma-separated addresses (IPv4:Port) of bootstrap peers")
	startLibrarianCmd.Flags().StringVarP(&logLevel, "log-level", "v", zap.InfoLevel.String(),
		"log level")

}

func getConfig() (*server.Config, *zap.Logger, error) {
	config := server.NewDefaultConfig()

	config.WithLocalAddr(server.ParseAddr(localIP, localPort))
	config.WithPublicAddr(server.ParseAddr(publicIP, publicPort))
	config.WithPublicName(publicName)
	config.WithDataDir(dataDir)
	config.WithDBDir(dbDir)
	config.WithLogLevel(getLogLevel())
	logger := clogging.NewDevLogger(config.LogLevel)

	bootstrapNetAddrs, err := server.ParseAddrs(bootstrapAddrs)
	if err != nil {
		logger.Error("unable to parse bootstrap peer address", zap.Error(err))
		return nil, logger, err

	}
	config.WithBootstrapAddrs(bootstrapNetAddrs)

	return config, logger, nil
}

func getLogLevel() zapcore.Level {
	var ll zapcore.Level
	err := ll.Set(logLevel)
	if err != nil {
		panic(err)
	}
	return ll
}
