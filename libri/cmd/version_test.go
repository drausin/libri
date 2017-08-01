package cmd

import "testing"

func TestVersionCmd(t *testing.T) {
	versionCmd.Run(versionCmd, []string{})
}
