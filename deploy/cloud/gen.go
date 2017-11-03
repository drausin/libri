package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hashicorp/terraform/helper/variables"
	"github.com/spf13/cobra"
)

const (
	k8sTemplateDir          = "kubernetes"
	defaultTemplateFilename = "libri.template.yml"
	defaultOutputFilename   = "libri.yml"
	defaultLocalCluster     = false
	defaultGCECluster       = false

	// Terraform variable keys
	tfNumLibrarians    = "num_librarians"
	tfPublicPortStart  = "librarian_public_port_start"
	tfLocalPort        = "librarian_local_port"
	tfLocalMetricsPort = "librarian_local_metrics_port"
)

var (
	templateFilepath string
	outFilepath      string
	tfvarsFilepath   string
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
	Use:   "go run gen.go",
	Short: "generate libri Kubernetes YML config",
	Long:  `generate libri Kubernetes YML config`,
	Run: func(cmd *cobra.Command, args []string) {
		if localCluster == gceCluster {
			fmt.Println("must either specify --local or --gce cluster, but not both")
			os.Exit(1)
		}
		if tfvarsFilepath == "" {
			fmt.Println("tfVarsFile is is required")
			os.Exit(1)
		}

		tfvars := make(variables.FlagFile)
		err := tfvars.Set(tfvarsFilepath)
		genMaybeExit(err)

		config := Config{
			LocalPort:        tfvars[tfLocalPort].(int),
			LocalMetricsPort: tfvars[tfLocalMetricsPort].(int),
			Librarians:       make([]Librarian, tfvars[tfNumLibrarians].(int)),
			LocalCluster:     localCluster,
			GCECluster:       gceCluster,
		}
		for i := range config.Librarians {
			config.Librarians[i].PublicPort = tfvars[tfPublicPortStart].(int) + i
		}

		wd, err := os.Getwd()
		genMaybeExit(err)
		absTemplateFilepath := filepath.Join(wd, k8sTemplateDir, templateFilepath)
		tmpl, err := template.New(templateFilepath).ParseFiles(absTemplateFilepath)
		genMaybeExit(err)

		absOutFilepath := filepath.Join(wd, outFilepath)
		out, err := os.Create(absOutFilepath)
		genMaybeExit(err)
		err = tmpl.Execute(out, config)
		genMaybeExit(err)
		fmt.Printf("wrote config to %s\n", outFilepath)
	},
}

func genMaybeExit(err error) {
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}

func init() {
	genCmd.Flags().StringVarP(&templateFilepath, "templateFile", "t", defaultTemplateFilename,
		"template YML filepath")
	genCmd.Flags().StringVarP(&outFilepath, "outFile", "o", defaultOutputFilename,
		"output YML filepath")
	genCmd.Flags().StringVarP(&tfvarsFilepath, "tfvarsFile", "v", "",
		"filepath of *.tfvars file defining properties (required)")
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
