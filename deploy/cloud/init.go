package main

import (
	"github.com/spf13/cobra"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const (
	templateDir          = "terraform/gce"
	mainTemplateFilename = "main.template.tf"
	mainFilename         = "main.tf"
	propsFilename        = "properties.tfvars"
)

type Config struct {
	ClusterName string
	Bucket      string
	GCPProject  string
	OutDir string
}

var flags Config

var initCmd = cobra.Command{
	Use:   "go run init.go",
	Short: "initialize flags for a new libri cluster",
	Long:  "initialize flags for a new libri cluster",
	Run: func(cmd *cobra.Command, args []string) {
		config := flags
		checkParams(config)

		absOutDir := filepath.Join(config.OutDir, flags.ClusterName)
		if _, err := os.Stat(absOutDir); os.IsNotExist(err) {
			err := os.Mkdir(absOutDir, os.ModePerm)
			maybeExit(err)
		}

		writeMainTFFile(config, absOutDir)
		writePropsFile(config, absOutDir)

		// - tf init

		fmt.Printf("initialized cluster flags in %s\n", flags.OutDir)
	},
}

func checkParams(config Config) {
	missingParam := false
	if config.OutDir == "" {
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

func writeMainTFFile(config Config, absOutDir string) {
	wd, err := os.Getwd()
	maybeExit(err)
	absMainTemplateFilepath := filepath.Join(wd, templateDir, mainTemplateFilename)
	mainTmpl, err := template.New(mainTemplateFilename).ParseFiles(absMainTemplateFilepath)
	maybeExit(err)

	absMainOutFilepath := filepath.Join(absOutDir, mainFilename)
	mainFile, err := os.Create(absMainOutFilepath)
	maybeExit(err)

	err = mainTmpl.Execute(mainFile, config)
	maybeExit(err)
}

func writePropsFile(config Config, absOutDir string) {
	wd, err := os.Getwd()
	maybeExit(err)
	absPropsTemplateFilepath := filepath.Join(wd, templateDir, propsFilename)
	propsTmpl, err := template.New(propsFilename).ParseFiles(absPropsTemplateFilepath)
	maybeExit(err)

	absPropsOutFilepath := filepath.Join(absOutDir, propsFilename)
	propsFile, err := os.Create(absPropsOutFilepath)
	maybeExit(err)

	err = propsTmpl.Execute(propsFile, config)
	maybeExit(err)
}

func maybeExit(err error) {
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}

func init() {
	initCmd.Flags().StringVarP(&flags.OutDir, "outDir", "d", "",
		"directory to create new cluster flags subdirectory in")
	initCmd.Flags().StringVarP(&flags.ClusterName, "clusterName", "n", "",
		"cluster name (without spaces)")
	initCmd.Flags().StringVarP(&flags.Bucket, "bucket", "b", "",
		"bucket where cluster state will be stored")
	initCmd.Flags().StringVarP(&flags.GCPProject, "gcpProject", "p", "",
		"GCP project to create infrastructure in")
}

func main() {
	if err := initCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
