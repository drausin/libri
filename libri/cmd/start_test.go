package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestStartLibrarianCmd_ok : happy path is kinda hard to test b/c it involves a long-running
// server, but this is tested in acceptance/local-demo.sh, so it's ok to skip here

func TestStartLibrarianCmd_err(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-librarian-data-dir")
	defer func() { err = os.RemoveAll(dataDir) }()
	viper.Set(dataDirFlag, dataDir)

	// check Start failure (from no bootstraps) bubbles up
	err = startLibrarianCmd.RunE(librarianCmd, []string{})
	assert.NotNil(t, err)
}

func TestGetLibrarianConfig_ok(t *testing.T) {
	publicIP := "1.2.3.4"
	localPort, localMetricsPort, localProfilerPort, publicPort := 1234, 1235, 1236, 6789
	profile := true
	publicName := "some name"
	dataDir := "some/data/dir"
	logLevel := "debug"
	nSubscriptions, fpRate := 5, 0.5
	bootstraps := "1.2.3.5:1000 1.2.3.6:1000"
	nBucketPeers := uint(8)
	verifyInterval := 5 * time.Second

	viper.Set(logLevelFlag, logLevel)
	viper.Set(publicHostFlag, publicIP)
	viper.Set(localPortFlag, localPort)
	viper.Set(localMetricsPortFlag, localMetricsPort)
	viper.Set(localProfilerPortFlag, localProfilerPort)
	viper.Set(profileFlag, profile)
	viper.Set(publicPortFlag, publicPort)
	viper.Set(publicNameFlag, publicName)
	viper.Set(dataDirFlag, dataDir)
	viper.Set(nSubscriptionsFlag, nSubscriptions)
	viper.Set(fpRateFlag, fpRate)
	viper.Set(bootstrapsFlag, bootstraps)
	viper.Set(maxBucketPeersFlag, nBucketPeers)
	viper.Set(verifyIntervalFlag, verifyInterval)

	config, logger, err := getLibrarianConfig()
	assert.Nil(t, err)
	assert.NotNil(t, logger)
	assert.Equal(t, localPort, config.LocalPort)
	assert.Equal(t, localMetricsPort, config.LocalMetricsPort)
	assert.Equal(t, localProfilerPort, config.LocalProfilerPort)
	assert.Equal(t, profile, config.Profile)
	assert.Equal(t, fmt.Sprintf("%s:%d", publicIP, publicPort), config.PublicAddr.String())
	assert.Equal(t, publicName, config.PublicName)
	assert.Equal(t, dataDir, config.DataDir)
	assert.Equal(t, dataDir+"/"+server.DBSubDir, config.DbDir)
	assert.Equal(t, logLevel, config.LogLevel.String())
	assert.Equal(t, uint32(nSubscriptions), config.SubscribeTo.NSubscriptions)
	assert.Equal(t, float32(fpRate), config.SubscribeTo.FPRate)
	assert.Equal(t, 2, len(config.BootstrapAddrs))
	assert.Equal(t, nBucketPeers, config.Routing.MaxBucketPeers)
	assert.Equal(t, verifyInterval, config.Replicate.VerifyInterval)

	assert.Nil(t, os.RemoveAll(config.DataDir))
}

func TestGetLibrarianConfig_err(t *testing.T) {
	viper.Set(publicHostFlag, "A.B.C.D") // bad
	config, logger, err := getLibrarianConfig()
	assert.NotNil(t, err)
	assert.Nil(t, config)
	assert.Nil(t, logger)

	// reset to ok value
	viper.Set(publicHostFlag, "1.2.3.4")

	viper.Set(bootstrapsFlag, "bad bootstrap")
	config, logger, err = getLibrarianConfig()
	assert.NotNil(t, err)
	assert.Nil(t, config)
	assert.Nil(t, logger)
}
