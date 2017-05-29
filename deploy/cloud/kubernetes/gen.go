package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"
	"text/template"
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

type Librarian struct {
	PublicPort int
}

type Config struct {
	LocalPort  int
	Librarians []Librarian
}

var GenCmd = &cobra.Command{
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
	GenCmd.Flags().StringVarP(&templateFilepath, "template-file", "t", defaultTemplateFilepath,
		"template YML filepath")
	GenCmd.Flags().StringVarP(&outFilepath, "out-file", "o", defaultOutputFilepath,
		"output YML filepath")
	GenCmd.Flags().IntVarP(&nReplicas, "n-replicas", "n", defaultNReplicas,
		"number of librarian replicas")
	GenCmd.Flags().IntVarP(&publicPortStart, "public-port", "p", defaultPublicPortStart,
		"starting public port for librarians")
	GenCmd.Flags().IntVarP(&localPort, "local-port", "l", defaultLocalPort,
		"local port for librarian node")
}

func main() {
	if err := GenCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
