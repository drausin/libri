package cmd

import (
	"testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"os"
	"io/ioutil"
)

// TestHealthCmd_ok : this path is annoying to test b/c it involves lots of setup; but this is
// tested in acceptance/local-demo.sh, so it's ok to skip here

func TestHealthCmd_err(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-author-data-dir")
	defer func() { err = os.RemoveAll(dataDir) }()
	viper.Set(dataDirFlag, dataDir)

	// check newTestAuthorGetter() error bubbles up
	viper.Set(testLibrariansFlag, "bad librarians address")
	err = healthCmd.RunE(healthCmd, []string{})
	assert.NotNil(t, err)

	// check Healthcheck() error from ok but missing librarians bubbles up
	viper.Set(testLibrariansFlag, "localhost:20200 localhost:20201")
	err = healthCmd.RunE(healthCmd, []string{})
	assert.NotNil(t, err)
}