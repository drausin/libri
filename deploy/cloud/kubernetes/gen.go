package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

const (
	defaultTemplateFilepath = "libri.template.yml"
	defaultOutputFilepath   = "libri.yml"
	defaultNReplicas        = 3
	defaultPublicPortStart  = 30100
	defaultLocalPort        = 20100
)

var (
	templateFilepath string
	outFilepath      string
	nReplicas        int
	publicPortStart  int
	localPort        int
)

// Config contains the configuration to apply to the template.
type Config struct {
	LocalPort  int
	Librarians []Librarian
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
			panic(err)
		}

		config := Config{
			LocalPort:  localPort,
			Librarians: make([]Librarian, nReplicas),
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
		"local port for librarian node")
}

func main() {
	if err := genCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
