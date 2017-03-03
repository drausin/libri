package cmd

import (
	"github.com/spf13/cobra"
	"github.com/drausin/libri/libri/librarian/server"
	"net"
)

// flags set below
var (
	peerName   string
	localName  string
	dataDir    string
	dbDir      string
	localIP    string
	localPort  int
	publicIP   string
	publicPort int
)

// startLibrarianCmd represents the librarian start command
var startLibrarianCmd = &cobra.Command{
	Use:   "start",
	Short: "start a librarian server",
	Long:  `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		server.Start(getConfig())
	},
}

func init() {
	librarianCmd.AddCommand(startLibrarianCmd)

	startLibrarianCmd.Flags().StringVarP(&peerName, "peer-name", "n", "",
		"public peer name")
	startLibrarianCmd.Flags().StringVarP(&localName, "local-name", "l", "",
		"local peer name")
	startLibrarianCmd.Flags().StringVarP(&dataDir, "data-dir", "d", "",
		"local data directory")
	startLibrarianCmd.Flags().StringVarP(&dbDir, "db-dir", "b", "",
		"local DB directory")

	startLibrarianCmd.Flags().StringVarP(&localIP, "local-ip", "i", server.DefaultIP,
		"local IP address")
	startLibrarianCmd.Flags().IntVarP(&localPort, "local-port", "p", server.DefaultPort,
		"local port")
	startLibrarianCmd.Flags().StringVarP(&publicIP, "public-ip", "j", server.DefaultIP,
		"public IP address")
	startLibrarianCmd.Flags().IntVarP(&publicPort, "public-port", "q", server.DefaultPort,
		"public port")
}

func getConfig() *server.Config {
	return &server.Config{
		PeerName: peerName,
		LocalName: localName,
		DataDir: dataDir,
		DbDir: dbDir,
		LocalAddr: &net.TCPAddr{IP: server.ParseIP(localIP), Port: localPort},
		PublicAddr: &net.TCPAddr{IP: server.ParseIP(publicIP), Port: publicPort},
	}
}
