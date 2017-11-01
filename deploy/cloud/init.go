package main

import (
	"github.com/spf13/cobra"
	"fmt"
	"os"
	"path/filepath"
	"log"
	"text/template"
)

const (
	mainTemplateDir      = "terraform/gce"
	mainTemplateFilename = "main.template.tf"
	mainFilename         = "main.tf"
)

type Config struct {
	ClusterName string
	Bucket      string
	GCPProject  string
}

var config Config
var outputDir string

var initCmd = cobra.Command{
	Use:   "go run init.go",
	Short: "initialize config for a new libri cluster",
	Long:  "initialize config for a new libri cluster",
	Run: func(cmd *cobra.Command, args []string) {
		checkParams()
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		absMainTemplateFilepath := filepath.Join(wd, mainTemplateDir, mainTemplateFilename)
		mainTmpl, err := template.New(mainTemplateFilename).ParseFiles(absMainTemplateFilepath)
		if err != nil {
			log.Fatal(err)
		}

		absOutDir := filepath.Join(outputDir, config.ClusterName)
		if _, err := os.Stat(absOutDir); os.IsNotExist(err) {
			if err := os.Mkdir(absOutDir,os.ModePerm); err != nil {
				log.Fatal(err)
			}
		}
		absOutMainFilepath := filepath.Join(absOutDir, mainFilename)
		mainFile, err := os.Create(absOutMainFilepath)
		if err != nil {
			log.Fatal(err)
		}
		err = mainTmpl.Execute(mainFile, config)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("initialized cluster config in %s\n", absOutDir)
	},
}

func checkParams() {
	missingParam := false
	if outputDir == "" {
		fmt.Println("outputDir parameteter is required")
		missingParam = true
	}
	if config.ClusterName == "" {
		fmt.Println("clusterName parameter is required")
		missingParam = true
	}
	if config.Bucket == "" {
		fmt.Println("bucket parameter is required")
		missingParam = true
	}
	if config.GCPProject == "" {
		fmt.Println("gcpProject parameter is required")
		missingParam = true
	}
	if missingParam {
		os.Exit(1)
	}
}

func init() {
	initCmd.Flags().StringVarP(&outputDir, "outDir", "d", "",
		"directory to create new cluster config subdirectory in")
	initCmd.Flags().StringVarP(&config.ClusterName, "clusterName", "n", "",
		"cluster name (without spaces)")
	initCmd.Flags().StringVarP(&config.Bucket, "bucket", "b", "",
		"bucket where cluster state will be stored")
	initCmd.Flags().StringVarP(&config.GCPProject, "gcpProject", "p", "",
		"GCP project to create infrastructure in")
}

func main() {
	if err := initCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
