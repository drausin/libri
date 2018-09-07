package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/parse"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/drausin/libri/libri/librarian/server/replicate"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	bootstrapsFlag        = "bootstraps"
	localPortFlag         = "localPort"
	localMetricsPortFlag  = "localMetricsPort"
	localProfilerPortFlag = "localProfilerPort"
	publicHostFlag        = "publicHost"
	publicNameFlag        = "publicName"
	publicPortFlag        = "publicPort"
	nSubscriptionsFlag    = "nSubscriptions"
	fpRateFlag            = "fpRate"
	profileFlag           = "profile"
	maxBucketPeersFlag    = "maxRoutingBucketPeers"
	verifyIntervalFlag    = "verifyInterval"
	organizationIDFlag    = "organizationID"

	logLocalPort        = "localPort"
	logLocalMetricsPort = "localMetricsPort"
	logPublicAddr       = "publicAddr"
)

// startLibrarianCmd represents the librarian start command
var startLibrarianCmd = &cobra.Command{
	Use:   "start",
	Short: "start a librarian server",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, logger, err := getLibrarianConfig()
		if err != nil {
			return err
		}
		up := make(chan *server.Librarian, 1)
		return server.Start(logger, config, up)
	},
}

func init() {
	librarianCmd.AddCommand(startLibrarianCmd)

	startLibrarianCmd.Flags().Int(localPortFlag, server.DefaultPort,
		"local port")
	startLibrarianCmd.Flags().Int(localMetricsPortFlag, server.DefaultMetricsPort,
		"local metrics port")
	startLibrarianCmd.Flags().Int(localProfilerPortFlag, server.DefaultProfilerPort,
		"local profiler port (when --profile is set)")
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
	startLibrarianCmd.Flags().Bool(profileFlag, false,
		"enable /debug/pprof profiler endpoint")
	startLibrarianCmd.Flags().Uint(maxBucketPeersFlag, routing.DefaultMaxActivePeers,
		"max number of peers allowed in a routing table bucket")
	startLibrarianCmd.Flags().Duration(verifyIntervalFlag, replicate.DefaultVerifyInterval,
		"verify interval duration")
	startLibrarianCmd.Flags().String(organizationIDFlag, "",
		"[sensitive] hex value of organization ID private key")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI") // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()        // read in environment variables that match
	errors.MaybePanic(viper.BindPFlags(startLibrarianCmd.Flags()))
}

func getLibrarianConfig() (*server.Config, *zap.Logger, error) {
	logLevel := getLogLevel()
	logger := clogging.NewDevLogger(logLevel)

	localPort := viper.GetInt(localPortFlag)
	localMetricsPort := viper.GetInt(localMetricsPortFlag)
	localProfilerPort := viper.GetInt(localProfilerPortFlag)
	profile := viper.GetBool(profileFlag)
	publicAddr, err := parse.Addr(
		viper.GetString(publicHostFlag),
		viper.GetInt(publicPortFlag),
	)
	if err != nil {
		logger.Error("fatal error parsing public address", zap.Error(err))
		return nil, nil, err
	}
	replicateParams := replicate.NewDefaultParameters()
	replicateParams.VerifyInterval = viper.GetDuration(verifyIntervalFlag)
	orgID, err := getOrgID(logger)
	if err != nil {
		return nil, nil, err
	}

	config := server.NewDefaultConfig().
		WithLocalPort(localPort).
		WithLocalMetricsPort(localMetricsPort).
		WithLocalProfilerPort(localProfilerPort).
		WithProfile(profile).
		WithPublicAddr(publicAddr).
		WithPublicName(viper.GetString(publicNameFlag)).
		WithOrgID(orgID).
		WithReplicate(replicateParams).
		WithDataDir(viper.GetString(dataDirFlag)).
		WithDefaultDBDir(). // depends on DataDir
		WithLogLevel(logLevel)
	config.SubscribeTo.NSubscriptions = uint32(viper.GetInt(nSubscriptionsFlag))
	config.SubscribeTo.FPRate = float32(viper.GetFloat64(fpRateFlag))
	config.Routing.MaxBucketPeers = uint(viper.GetInt(maxBucketPeersFlag))

	bootstrapNetAddrs, err := parse.Addrs(viper.GetStringSlice(bootstrapsFlag))
	if err != nil {
		logger.Error("unable to parse bootstrap peer address", zap.Error(err))
		return nil, nil, err

	}
	config.WithBootstrapAddrs(bootstrapNetAddrs)

	WriteLibrarianBanner(os.Stdout)
	logger.Info("librarian configuration",
		zap.Int(logLocalPort, config.LocalPort),
		zap.Int(logLocalMetricsPort, config.LocalMetricsPort),
		zap.Stringer(logPublicAddr, config.PublicAddr),
		zap.String(bootstrapsFlag, fmt.Sprintf("%v", config.BootstrapAddrs)),
		zap.String(publicNameFlag, config.PublicName),
		zap.String(dataDirFlag, config.DataDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
		zap.Uint32(nSubscriptionsFlag, config.SubscribeTo.NSubscriptions),
		zap.Float32(fpRateFlag, config.SubscribeTo.FPRate),
		zap.Uint(maxBucketPeersFlag, config.Routing.MaxBucketPeers),
	)
	return config, logger, nil
}

func getOrgID(logger *zap.Logger) (ecid.ID, error) {
	orgIDPrivHex := viper.GetString(organizationIDFlag)
	if len(orgIDPrivHex) == 0 {
		// ok if org ID isn't set
		return nil, nil
	}
	orgIDPrivBytes, err := hex.DecodeString(strings.TrimSpace(orgIDPrivHex))
	if err != nil {
		logger.Error("fatal error parsing organization ID private key hex")
		return nil, err
	}
	expectedByteLen := ecid.Curve.BitSize / 8
	if len(orgIDPrivBytes) != expectedByteLen {
		logger.Error("organization ID private key hex is not the expected length",
			zap.Int("expected_length", expectedByteLen),
			zap.Int("actual_length", len(orgIDPrivBytes)),
		)
		return nil, err
	}
	priv, err := crypto.ToECDSA(orgIDPrivBytes)
	if err != nil {
		logger.Error("unable to construct organization ID private key")
		return nil, err
	}
	return ecid.FromPrivateKey(priv)
}
