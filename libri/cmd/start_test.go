package cmd

import (
	"io/ioutil"
	"os"
	"testing"

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

	// check getLibrarianConfig error bubbles up
	viper.Set(localHostFlag, "bad local host")
	err = startLibrarianCmd.RunE(startLibrarianCmd, []string{})
	assert.NotNil(t, err)

	// reset to ok value
	viper.Set(localHostFlag, "1.2.3.4")

	// check Start failure (from no bootstraps) bubbles up
	err = startLibrarianCmd.RunE(librarianCmd, []string{})
	assert.NotNil(t, err)
}

func TestGetLibrarianConfig_ok(t *testing.T) {
	localIP, publicIP := "1.2.3.4", "5.6.7.8"
	localPort, localMetricsPort, publicPort := "1234", "1235", "6789"
	publicName := "some name"
	dataDir := "some/data/dir"
	logLevel := "debug"
	nSubscriptions, fpRate := 5, 0.5
	bootstraps := "1.2.3.5:1000 1.2.3.6:1000"

	viper.Set(logLevelFlag, logLevel)
	viper.Set(localHostFlag, localIP)
	viper.Set(publicHostFlag, publicIP)
	viper.Set(localPortFlag, localPort)
	viper.Set(localMetricsPortFlag, localMetricsPort)
	viper.Set(publicPortFlag, publicPort)
	viper.Set(publicNameFlag, publicName)
	viper.Set(dataDirFlag, dataDir)
	viper.Set(nSubscriptionsFlag, nSubscriptions)
	viper.Set(fpRateFlag, fpRate)
	viper.Set(bootstrapsFlag, bootstraps)

	config, logger, err := getLibrarianConfig()
	assert.Nil(t, err)
	assert.NotNil(t, logger)
	assert.Equal(t, localIP+":"+localPort, config.LocalAddr.String())
	assert.Equal(t, localIP+":"+localMetricsPort, config.LocalMetricsAddr.String())
	assert.Equal(t, publicIP+":"+publicPort, config.PublicAddr.String())
	assert.Equal(t, publicName, config.PublicName)
	assert.Equal(t, dataDir, config.DataDir)
	assert.Equal(t, dataDir+"/"+server.DBSubDir, config.DbDir)
	assert.Equal(t, logLevel, config.LogLevel.String())
	assert.Equal(t, uint32(nSubscriptions), config.SubscribeTo.NSubscriptions)
	assert.Equal(t, float32(fpRate), config.SubscribeTo.FPRate)
	assert.Equal(t, 2, len(config.BootstrapAddrs))

	assert.Nil(t, os.RemoveAll(config.DataDir))
}

func TestGetLibrarianConfig_err(t *testing.T) {
	viper.Set(localHostFlag, "bad local host")
	config, logger, err := getLibrarianConfig()
	assert.NotNil(t, err)
	assert.Nil(t, config)
	assert.Nil(t, logger)

	// reset to ok value
	viper.Set(localHostFlag, "1.2.3.4")

	viper.Set(localMetricsPortFlag, -1)
	config, logger, err = getLibrarianConfig()
	assert.NotNil(t, err)
	assert.Nil(t, config)
	assert.Nil(t, logger)

	// reset to ok value
	viper.Set(localMetricsPortFlag, 1234)

	viper.Set(publicHostFlag, "bad public host")
	config, logger, err = getLibrarianConfig()
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
