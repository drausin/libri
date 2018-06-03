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
	tfGCETemplateDir      = "terraform/gcp"
	tfMinikubeTemplateDir = "terraform/minikube"
	mainTemplateFilename  = "main.template.tf"
	varsFilename          = "variables.tf"
	mainFilename          = "main.tf"
	defaultFlagsFilename  = "terraform.tfvars"
	moduleSubDir          = "module"

	kubeTemplateDir            = "kubernetes"
	kubeConfigTemplateFilename = "libri.template.yml"
	kubeConfigFilename         = "libri.yml"

	// Terraform variable keys
	tfClusterAdminUser      = "cluster_admin_user"
	tfClusterHost           = "cluster_host"
	tfNumLibrarians         = "num_librarians"
	tfLibrarianLibriVersion = "librarian_libri_version"
	tfLibrarianCPULimit     = "librarian_cpu_limit"
	tfLibrarianRAMLimit     = "librarian_ram_limit"
	tfLibrarianDiskSizeGB   = "librarian_disk_size_gb"
	tfPublicPortStart       = "librarian_public_port_start"
	tfLocalPort             = "librarian_local_port"
	tfLocalMetricsPort      = "librarian_local_metrics_port"
	tfGrafanaPort           = "grafana_port"
	tfPrometheusPort        = "prometheus_port"
	tfGrafanaCPULimit       = "grafana_cpu_limit"
	tfGrafanaRAMLimit       = "grafana_ram_limit"
	tfPrometheusCPULimit    = "prometheus_cpu_limit"
	tfPrometheusRAMLimit    = "prometheus_ram_limit"

	tfClusterHostGCP      = "gcp"
	tfClusterHostMinikube = "minikube"
)

// TFConfig defines the configuration of the Terraform infrastructure.
type TFConfig struct {
	ClusterName     string
	Bucket          string
	GCPProject      string
	ClusterDir      string
	LocalModulePath string
	FlagsFilepath   string
}

// KubeConfig contains the configuration to apply to the template.
type KubeConfig struct {
	ClusterAdminUser    string
	LibriVersion        string
	LocalPort           int
	LocalMetricsPort    int
	GrafanaPort         int
	PrometheusPort      int
	GrafanaCPULimit     string
	PrometheusCPULimit  string
	GrafanaRAMLimit     string
	PrometheusRAMLimit  string
	Librarians          []LibrarianConfig
	LibrarianCPULimit   string
	LibrarianRAMLimit   string
	LibrarianDiskSizeGB int
	LocalCluster        bool
	GCPCluster          bool
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
	initCmd.PersistentFlags().StringVarP(&initFlags.ClusterDir, "clusterDir", "d", "",
		"directory to create new cluster in")
	initCmd.PersistentFlags().StringVarP(&initFlags.ClusterName, "clusterName", "n", "",
		"cluster name (without spaces)")
	initCmd.PersistentFlags().StringVarP(&initFlags.FlagsFilepath, "flagsFilepath", "f", "",
		"(optional) filepath of terraform.tfvars file")

	initCmd.AddCommand(minikubeCmd)
	initCmd.AddCommand(gcpCmd)
	gcpCmd.Flags().StringVarP(&initFlags.Bucket, "bucket", "b", "",
		"bucket where Terraform remote cluster state will be stored")
	gcpCmd.Flags().StringVarP(&initFlags.GCPProject, "gcpProject", "p", "",
		"GCP project to create infrastructure in")

	clusterCmd.AddCommand(planCmd)
	planCmd.Flags().StringVarP(&clusterDir, "clusterDir", "d", "",
		"local cluster directory (required)")
	planCmd.Flags().BoolVarP(&notf, "notf", "", false, "skip Terraform planning")
	planCmd.Flags().BoolVarP(&nokube, "nokube", "", false, "skip Kubernetes planning")

	clusterCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringVarP(&clusterDir, "clusterDir", "d", "",
		"local cluster directory (required)")
	applyCmd.Flags().BoolVarP(&notf, "notf", "", false, "skip Terraform applying")
	applyCmd.Flags().BoolVarP(&nokube, "nokube", "", false, "skip Kubernetes applying")
}

var clusterCmd = &cobra.Command{
	Short: "operate a libri cluster",
	Long:  "operate a libri cluster",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new libri cluster",
	Long:  "initialize a new libri cluster",
}

var minikubeCmd = &cobra.Command{
	Use:   "minikube",
	Short: "operate a local minikube cluster",
	Long:  "operate a local minikube cluster",
	Run: func(cmd *cobra.Command, args []string) {
		config := initFlags
		checkInitMinikubeParams(config)

		maybeMkdir(config.ClusterDir)
		// TF infra can't be applied to minikube, so no main.tf is written
		writeVarsTFFile(config, config.ClusterDir, tfMinikubeTemplateDir)
		writePropsFile(config, config.ClusterDir, tfMinikubeTemplateDir)

		fmt.Printf("%s successfully initialized in %s\n", config.ClusterName, config.ClusterDir)
	},
}

var gcpCmd = &cobra.Command{
	Use:   "gcp",
	Short: "operate a Google Cloud Platform cluster",
	Long:  "operate a Google Cloud Platform cluster",
	Run: func(cmd *cobra.Command, args []string) {
		config := initFlags
		checkInitGCPParams(config)

		maybeMkdir(config.ClusterDir)
		writeMainTFFile(config, config.ClusterDir, tfGCETemplateDir)
		writeVarsTFFile(config, config.ClusterDir, tfGCETemplateDir)
		writePropsFile(config, config.ClusterDir, tfGCETemplateDir)
		tfCommand(config.ClusterDir, "init")

		fmt.Printf("\n%s successfully initialized in %s\n", config.ClusterName, config.ClusterDir)
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
		if tfvars := getTFFlags(clusterDir); tfvars[tfClusterHost] == tfClusterHostMinikube {
			// skip TF if it's a minikube cluster
			notf = true
		}

		if !notf {
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
		if tfvars := getTFFlags(clusterDir); tfvars[tfClusterHost] == tfClusterHostMinikube {
			// skip TF if it's a minikube cluster
			notf = true
		}
		if !notf {
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

func checkInitGCPParams(config TFConfig) {
	missingParam := false
	if config.ClusterDir == "" {
		fmt.Println("clusterDir parameteter is required")
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

func checkInitMinikubeParams(config TFConfig) {
	missingParam := false
	if config.ClusterDir == "" {
		fmt.Println("clusterDir parameteter is required")
		missingParam = true
	}
	if config.ClusterName == "" {
		fmt.Println("clusterName parameter is required")
		missingParam = true
	}
	if missingParam {
		os.Exit(1)
	}
}

func writeMainTFFile(config TFConfig, clusterDir, templateDir string) {
	wd, err := os.Getwd()
	maybeExit(err)

	config.LocalModulePath = filepath.Join(wd, templateDir, moduleSubDir)
	absMainTemplateFilepath := filepath.Join(wd, templateDir, mainTemplateFilename)
	mainTmpl, err := template.New(mainTemplateFilename).ParseFiles(absMainTemplateFilepath)
	maybeExit(err)

	absMainOutFilepath := filepath.Join(clusterDir, mainFilename)
	mainFile, err := os.Create(absMainOutFilepath)
	maybeExit(err)

	err = mainTmpl.Execute(mainFile, config)
	maybeExit(err)
}

func writeVarsTFFile(config TFConfig, clusterDir, templateDir string) {
	wd, err := os.Getwd()
	maybeExit(err)

	absVarsFilepath := filepath.Join(wd, templateDir, varsFilename)
	varsTmpl, err := template.New(varsFilename).ParseFiles(absVarsFilepath)
	maybeExit(err)

	absVarsOutFilepath := filepath.Join(clusterDir, varsFilename)
	varsFile, err := os.Create(absVarsOutFilepath)
	maybeExit(err)

	err = varsTmpl.Execute(varsFile, config)
	maybeExit(err)
}

func writePropsFile(config TFConfig, clusterDir, templateDir string) {
	wd, err := os.Getwd()
	maybeExit(err)
	absPropsTemplateFilepath := config.FlagsFilepath
	if absPropsTemplateFilepath == "" {
		absPropsTemplateFilepath = filepath.Join(wd, templateDir, defaultFlagsFilename)
	}
	propsTmpl, err := template.New(defaultFlagsFilename).ParseFiles(absPropsTemplateFilepath)
	maybeExit(err)

	absPropsOutFilepath := filepath.Join(clusterDir, defaultFlagsFilename)
	propsFile, err := os.Create(absPropsOutFilepath)
	maybeExit(err)

	err = propsTmpl.Execute(propsFile, config)
	maybeExit(err)
}

func tfCommand(clusterDir string, subcommand string) {
	tfInitCmd := exec.Command("terraform", subcommand) // nolint: gas
	tfInitCmd.Stdin = os.Stdin
	tfInitCmd.Stdout = os.Stdout
	tfInitCmd.Stderr = os.Stderr
	tfInitCmd.Dir = clusterDir
	err := tfInitCmd.Run()
	maybeExit(err)
}

func kubeApply(clusterDir string, dryRun bool) {
	tfInitCmd := exec.Command("kubectl", "apply", "-f", kubeConfigFilename) // nolint: gas
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
	tfvars := getTFFlags(clusterDir)
	config := KubeConfig{
		ClusterAdminUser:   tfvars[tfClusterAdminUser].(string),
		LibriVersion:       tfvars[tfLibrarianLibriVersion].(string),
		LocalPort:          tfvars[tfLocalPort].(int),
		LocalMetricsPort:   tfvars[tfLocalMetricsPort].(int),
		GrafanaPort:        tfvars[tfGrafanaPort].(int),
		PrometheusPort:     tfvars[tfPrometheusPort].(int),
		Librarians:         make([]LibrarianConfig, tfvars[tfNumLibrarians].(int)),
		LibrarianCPULimit:  tfvars[tfLibrarianCPULimit].(string),
		LibrarianRAMLimit:  tfvars[tfLibrarianRAMLimit].(string),
		GrafanaCPULimit:    tfvars[tfGrafanaCPULimit].(string),
		GrafanaRAMLimit:    tfvars[tfGrafanaRAMLimit].(string),
		PrometheusCPULimit: tfvars[tfPrometheusCPULimit].(string),
		PrometheusRAMLimit: tfvars[tfPrometheusRAMLimit].(string),
		LocalCluster:       tfvars[tfClusterHost] == tfClusterHostMinikube,
		GCPCluster:         tfvars[tfClusterHost] == tfClusterHostGCP,
	}
	if value, in := tfvars[tfLibrarianDiskSizeGB]; in {
		config.LibrarianDiskSizeGB = value.(int)
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

func getTFFlags(clusterDir string) variables.FlagFile {
	tfvarsFilepath := path.Join(clusterDir, defaultFlagsFilename)
	tfvars := make(variables.FlagFile)
	err := tfvars.Set(tfvarsFilepath)
	maybeExit(err)
	return tfvars
}

func writeKubeConfigMaps() {
	maybeDelete := exec.Command("sh", "-c", "kubectl get configmap --no-headers | "+ // nolint: gas
		"grep 'config' | awk '{print $1}' | xargs -I {} kubectl delete configmap {}")
	maybeDelete.Stdout = os.Stdout
	err := maybeDelete.Run()
	maybeExit(err)

	resourceNames := []string{"prometheus", "grafana"}
	for _, name := range resourceNames {
		configName := fmt.Sprintf("%s-config", name)
		fromFile := fmt.Sprintf("--from-file=kubernetes/config/%s", name)
		create := exec.Command("kubectl", "create", "configmap", configName, fromFile) // nolint: gas
		create.Stdout = os.Stdout
		err = create.Run()
		maybeExit(err)
	}
}

func maybeMkdir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.Mkdir(dir, os.ModePerm)
		maybeExit(err)
	}
}

func maybeExit(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
