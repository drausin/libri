package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"math/rand"

	"encoding/hex"

	"strings"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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
	rng := rand.New(rand.NewSource(0))
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
	orgID := ecid.NewPseudoRandom(rng)
	orgIDHex := hex.EncodeToString(orgID.Key().D.Bytes())

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
	viper.Set(organizationIDFlag, orgIDHex)

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
	assert.Equal(t, orgID.Key(), config.OrgID.Key())

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

func TestGetOrgID_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	orgID1 := ecid.NewPseudoRandom(rng)
	orgIDHex := hex.EncodeToString(orgID1.Key().D.Bytes())
	lg := zap.NewNop()

	// no org ID set
	viper.Set(organizationIDFlag, "")
	orgID2, err := getOrgID(lg)
	assert.Nil(t, orgID2)
	assert.Nil(t, err)

	// org ID set
	viper.Set(organizationIDFlag, orgIDHex)
	orgID2, err = getOrgID(lg)
	assert.Equal(t, orgID1.Key(), orgID2.Key())
	assert.Nil(t, err)
}

func TestGetOrgID_err(t *testing.T) {
	lg := zap.NewNop()
	badOrgIDHexs := map[string]string{
		"not hex":      "not hex",
		"wrong length": strings.Repeat("a", 65),
		"not on curve": strings.Repeat("0", 64),
	}

	for desc, badOrgID := range badOrgIDHexs {
		viper.Set(organizationIDFlag, badOrgID)
		orgID, err := getOrgID(lg)
		assert.Nil(t, orgID, desc)
		assert.NotNil(t, err, desc)
	}
}
