package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/drausin/libri/libri/librarian/server"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	defaultTemplateFilepath = "libri.template.yml"
	defaultOutputFilepath   = "libri.yml"
	defaultNReplicas        = 3
	defaultPublicPortStart  = 30100
	defaultLocalPort        = server.DefaultPort
	defaultLocalMetricsPort = server.DefaultMetricsPort
	defaultLocalCluster     = false
	defaultGCECluster       = false
)

var (
	templateFilepath string
	outFilepath      string
	nReplicas        int
	publicPortStart  int
	localPort        int
	localMetricsPort int
	localCluster     bool
	gceCluster       bool
)

// Config contains the configuration to apply to the template.
type Config struct {
	LocalPort        int
	LocalMetricsPort int
	Librarians       []Librarian
	LocalCluster     bool
	GCECluster       bool
}

// Librarian contains the public-facing configuration for an individual librarian.
type Librarian struct {
	PublicPort int
}

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "generate libri Kubernetes YML config",
	Long:  `generate libri Kubernetes YML config`,
	Run: func(cmd *cobra.Command, args []string) {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		absTemplateFilepath := filepath.Join(wd, templateFilepath)
		tmpl, err := template.New(templateFilepath).ParseFiles(absTemplateFilepath)
		if err != nil {
			log.Fatal(err)
		}
		if localCluster == gceCluster {
			log.Fatal(errors.New("must either specify --local or --gce cluster, but not both"))
		}

		config := Config{
			LocalPort:        localPort,
			LocalMetricsPort: localMetricsPort,
			Librarians:       make([]Librarian, nReplicas),
			LocalCluster:     localCluster,
			GCECluster:       gceCluster,
		}
		for i := range config.Librarians {
			config.Librarians[i].PublicPort = publicPortStart + i
		}

		absOutFilepath := filepath.Join(wd, outFilepath)
		out, err := os.Create(absOutFilepath)
		if err != nil {
			panic(err)
		}
		if err = tmpl.Execute(out, config); err != nil {
			panic(err)
		}
		fmt.Printf("wrote config to %s\n", outFilepath)
	},
}

func init() {
	genCmd.Flags().StringVarP(&templateFilepath, "templateFile", "t", defaultTemplateFilepath,
		"template YML filepath")
	genCmd.Flags().StringVarP(&outFilepath, "outFile", "o", defaultOutputFilepath,
		"output YML filepath")
	genCmd.Flags().IntVarP(&nReplicas, "nReplicas", "n", defaultNReplicas,
		"number of librarian replicas")
	genCmd.Flags().IntVarP(&publicPortStart, "publicPort", "p", defaultPublicPortStart,
		"starting public port for librarians")
	genCmd.Flags().IntVarP(&localPort, "localPort", "l", defaultLocalPort,
		"localCluster port for librarian node")
	genCmd.Flags().IntVarP(&localMetricsPort, "localMetricsPort", "m", defaultLocalMetricsPort,
		"local metrics port for librarian node")
	genCmd.Flags().BoolVar(&localCluster, "local", defaultLocalCluster,
		"whether the config is for a local (minikube) cluster")
	genCmd.Flags().BoolVar(&gceCluster, "gce", defaultGCECluster,
		"whether the config is for a remote Google Compute Engine (GCE) cluster")
}

func main() {
	if err := genCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
