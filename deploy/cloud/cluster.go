package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/hashicorp/terraform/helper/variables"
	"github.com/spf13/cobra"
)

const (
	tfTemplateDir        = "terraform/gce"
	mainTemplateFilename = "main.template.tf"
	mainFilename         = "main.tf"
	propsFilename        = "terraform.tfvars"
	moduleSubDir         = "module"

	kubeTemplateDir            = "kubernetes"
	kubeConfigTemplateFilename = "libri.template.yml"
	kubeConfigFilename         = "libri.yml"

	// Terraform variable keys
	tfNumLibrarians    = "num_librarians"
	tfPublicPortStart  = "librarian_public_port_start"
	tfLocalPort        = "librarian_local_port"
	tfLocalMetricsPort = "librarian_local_metrics_port"
)

// TFConfig defines the configuration of the Terraform infrastructure.
type TFConfig struct {
	ClusterName     string
	Bucket          string
	GCPProject      string
	OutDir          string
	LocalModulePath string
}

// KubeConfig contains the configuration to apply to the template.
type KubeConfig struct {
	LocalPort        int
	LocalMetricsPort int
	Librarians       []LibrarianConfig
	LocalCluster     bool
	GCECluster       bool
}

// LibrarianConfig contains the public-facing configuration for an individual librarian.
type LibrarianConfig struct {
	PublicPort int
}

var (
	initFlags  TFConfig
	clusterDir string
	notf       bool
	nokube     bool
	minikube   bool
)

func main() {
	if err := clusterCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

// not to be confused with the init command
func init() {
	clusterCmd.AddCommand(initCmd)
	clusterCmd.AddCommand(planCmd)
	clusterCmd.AddCommand(applyCmd)

	initCmd.Flags().StringVarP(&initFlags.OutDir, "outDir", "d", "",
		"directory to create new cluster subdirectory in")
	initCmd.Flags().StringVarP(&initFlags.ClusterName, "clusterName", "n", "",
		"cluster clusterName (without spaces)")
	initCmd.Flags().StringVarP(&initFlags.Bucket, "bucket", "b", "none",
		"bucket where cluster state will be stored")
	initCmd.Flags().StringVarP(&initFlags.GCPProject, "gcpProject", "p", "none",
		"GCP project to create infrastructure in")

	planCmd.Flags().StringVarP(&clusterDir, "clusterDir", "c", "", "local cluster directory (required)")
	planCmd.Flags().BoolVarP(&notf, "notf", "", false, "skip Terraform planning")
	planCmd.Flags().BoolVarP(&nokube, "nokube", "", false, "skip Kubernetes planning")
	planCmd.Flags().BoolVarP(&minikube, "minikube", "", false,
		"use local minikube instead of Terraform intrastructure")

	applyCmd.Flags().StringVarP(&clusterDir, "clusterDir", "c", "", "local cluster directory (required)")
	applyCmd.Flags().BoolVarP(&notf, "notf", "", false, "skip Terraform applying")
	applyCmd.Flags().BoolVarP(&nokube, "nokube", "", false, "skip Kubernetes applying")
	applyCmd.Flags().BoolVarP(&minikube, "minikube", "", false,
		"use local minikube instead of Terraform intrastructure")
}

var clusterCmd = &cobra.Command{
	Short: "operate a libri cluster",
	Long:  "operate a libri cluster",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new libri cluster",
	Long:  "initialize a new libri cluster",
	Run: func(cmd *cobra.Command, args []string) {
		config := initFlags
		checkInitParams(config)

		clusterDir := filepath.Join(config.OutDir, initFlags.ClusterName)
		if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
			err := os.Mkdir(clusterDir, os.ModePerm)
			maybeExit(err)
		}

		writeMainTFFile(config, clusterDir)
		writePropsFile(config, clusterDir)
		tfCommand(clusterDir, "init")

		fmt.Printf("\n%s successfully initialized in %s\n", config.ClusterName, clusterDir)
	},
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "plan changes to a libri cluster",
	Long:  "plan changes to a libri cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if clusterDir == "" {
			maybeExit(errors.New("clusterDir param must be set"))
		}
		if !notf && !minikube {
			fmt.Printf("planning Terraform changes\n\n")
			tfCommand(clusterDir, "plan")
		}
		if !nokube {
			fmt.Printf("planning Kubernetes changes\n\n")
			writeKubeConfig(clusterDir)
			kubeApply(clusterDir, true)
		}
	},
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "apply changes to a libri cluster",
	Long:  "apply changes to a libri cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if clusterDir == "" {
			maybeExit(errors.New("clusterDir param must be set"))
		}
		if !notf && !minikube {
			fmt.Printf("applying Terraform changes\n\n")
			tfCommand(clusterDir, "apply")
		}
		if !nokube {
			fmt.Printf("applying Kubernetes changes\n\n")
			writeKubeConfig(clusterDir)
			writeKubeConfigMaps()
			kubeApply(clusterDir, false)
		}
	},
}

func checkInitParams(config TFConfig) {
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

func writeMainTFFile(config TFConfig, clusterDir string) {
	wd, err := os.Getwd()
	maybeExit(err)

	config.LocalModulePath = filepath.Join(wd, tfTemplateDir, moduleSubDir)
	absMainTemplateFilepath := filepath.Join(wd, tfTemplateDir, mainTemplateFilename)
	mainTmpl, err := template.New(mainTemplateFilename).ParseFiles(absMainTemplateFilepath)
	maybeExit(err)

	absMainOutFilepath := filepath.Join(clusterDir, mainFilename)
	mainFile, err := os.Create(absMainOutFilepath)
	maybeExit(err)

	err = mainTmpl.Execute(mainFile, config)
	maybeExit(err)
}

func writePropsFile(config TFConfig, clusterDir string) {
	wd, err := os.Getwd()
	maybeExit(err)
	absPropsTemplateFilepath := filepath.Join(wd, tfTemplateDir, propsFilename)
	propsTmpl, err := template.New(propsFilename).ParseFiles(absPropsTemplateFilepath)
	maybeExit(err)

	absPropsOutFilepath := filepath.Join(clusterDir, propsFilename)
	propsFile, err := os.Create(absPropsOutFilepath)
	maybeExit(err)

	err = propsTmpl.Execute(propsFile, config)
	maybeExit(err)
}

func tfCommand(clusterDir string, subcommand string) {
	tfInitCmd := exec.Command("terraform", subcommand)
	tfInitCmd.Stdin = os.Stdin
	tfInitCmd.Stdout = os.Stdout
	tfInitCmd.Stderr = os.Stderr
	tfInitCmd.Dir = clusterDir
	err := tfInitCmd.Run()
	maybeExit(err)
}

func kubeApply(clusterDir string, dryRun bool) {
	tfInitCmd := exec.Command("kubectl", "apply", "-f", kubeConfigFilename)
	if dryRun {
		tfInitCmd.Args = append(tfInitCmd.Args, "--dry-run")
	}
	tfInitCmd.Stdin = os.Stdin
	tfInitCmd.Stdout = os.Stdout
	tfInitCmd.Stderr = os.Stderr
	tfInitCmd.Dir = clusterDir
	err := tfInitCmd.Run()
	maybeExit(err)
}

func writeKubeConfig(clusterDir string) {
	tfvarsFilepath := path.Join(clusterDir, propsFilename)
	tfvars := make(variables.FlagFile)
	err := tfvars.Set(tfvarsFilepath)
	maybeExit(err)

	config := KubeConfig{
		LocalPort:        tfvars[tfLocalPort].(int),
		LocalMetricsPort: tfvars[tfLocalMetricsPort].(int),
		Librarians:       make([]LibrarianConfig, tfvars[tfNumLibrarians].(int)),
		LocalCluster:     minikube,
		GCECluster:       !minikube,
	}
	for i := range config.Librarians {
		config.Librarians[i].PublicPort = tfvars[tfPublicPortStart].(int) + i
	}

	wd, err := os.Getwd()
	maybeExit(err)
	templateFilename := filepath.Base(kubeConfigTemplateFilename)
	absTemplateFilepath := filepath.Join(wd, kubeTemplateDir, kubeConfigTemplateFilename)
	tmpl, err := template.New(templateFilename).ParseFiles(absTemplateFilepath)
	maybeExit(err)

	kubeConfigFilepath := path.Join(clusterDir, kubeConfigFilename)
	out, err := os.Create(kubeConfigFilepath)
	maybeExit(err)
	err = tmpl.Execute(out, config)
	maybeExit(err)
}

func writeKubeConfigMaps() {
	maybeDelete := exec.Command("sh", "-c", "kubectl get configmap --no-headers | "+
		"grep 'config' | awk '{print $1}' | xargs -I {} kubectl delete configmap {}")
	maybeDelete.Stdout = os.Stdout
	err := maybeDelete.Run()
	maybeExit(err)

	resourceNames := []string{"prometheus", "grafana"}
	for _, name := range resourceNames {
		configName := fmt.Sprintf("%s-config", name)
		fromFile := fmt.Sprintf("--from-file=kubernetes/config/%s", name)
		create := exec.Command("kubectl", "create", "configmap", configName, fromFile)
		create.Stdout = os.Stdout
		err = create.Run()
		maybeExit(err)
	}
}

func maybeExit(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
