package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

const (
	tfTemplateDir        = "terraform/gce"
	mainTemplateFilename = "main.template.tf"
	mainFilename         = "main.tf"
	propsFilename        = "terraform.tfvars"
	moduleSubDir         = "module"
)

// TFConfig defines the configuration of the Terraform infrastructure.
type TFConfig struct {
	ClusterName     string
	Bucket          string
	GCPProject      string
	OutDir          string
	LocalModulePath string
}

var flags TFConfig

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
			initMaybeExit(err)
		}

		writeMainTFFile(config, absOutDir)
		writePropsFile(config, absOutDir)

		fmt.Printf("initialized cluster terraform in %s\n", flags.OutDir)
		fmt.Println("To complete initialization, run the following in your shell:")
		fmt.Println()
		fmt.Printf("\tpushd %s && terraform init && popd\n", absOutDir)
		fmt.Println()
	},
}

func checkParams(config TFConfig) {
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

func writeMainTFFile(config TFConfig, absOutDir string) {
	wd, err := os.Getwd()
	initMaybeExit(err)

	config.LocalModulePath = filepath.Join(wd, tfTemplateDir, moduleSubDir)
	absMainTemplateFilepath := filepath.Join(wd, tfTemplateDir, mainTemplateFilename)
	mainTmpl, err := template.New(mainTemplateFilename).ParseFiles(absMainTemplateFilepath)
	initMaybeExit(err)

	absMainOutFilepath := filepath.Join(absOutDir, mainFilename)
	mainFile, err := os.Create(absMainOutFilepath)
	initMaybeExit(err)

	err = mainTmpl.Execute(mainFile, config)
	initMaybeExit(err)
}

func writePropsFile(config TFConfig, absOutDir string) {
	wd, err := os.Getwd()
	initMaybeExit(err)
	absPropsTemplateFilepath := filepath.Join(wd, tfTemplateDir, propsFilename)
	propsTmpl, err := template.New(propsFilename).ParseFiles(absPropsTemplateFilepath)
	initMaybeExit(err)

	absPropsOutFilepath := filepath.Join(absOutDir, propsFilename)
	propsFile, err := os.Create(absPropsOutFilepath)
	initMaybeExit(err)

	err = propsTmpl.Execute(propsFile, config)
	initMaybeExit(err)
}

func initMaybeExit(err error) {
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
