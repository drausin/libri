
terraform {
  backend "gcs" {
    bucket  = "{{ .Bucket }}"
    path    = "{{ .ClusterName }}/terraform/current.tfstate"
    project = "{{ .GCPProject }}"
  }
}

variable "credentials_file" {
  description = "GCP JSON credentials filepath"
}

variable "num_librarians" {
  description = "current number of librarian peers in cluster"
}

variable "librarian_disk_size_gb" {
  description = "size (GB) of persistant disk used by each librarian"
}

variable "librarian_libri_version" {
  description = "libri version (e.g., 0.1.0, latest, snapshot) to use for librarian container"
}

variable "librarian_cpu_limit" {
  description = "librarian container CPU limit (e.g., 500m, 0.5, 1)"
}

variable "librarian_ram_limit" {
  description = "librarian container RAM limit (e.g., 500M, 1G)"
}

variable "num_cluster_nodes" {
  description = "current number of cluster nodes"
}

variable "cluster_node_machine_type" {
  description = "GCE cluster node machine type"
}

variable "librarian_public_port_start" {
  description = "public port for librarian-0 service"
}

variable "librarian_local_port" {
  description = "local port for each librarian instance"
}

variable "librarian_local_metrics_port" {
  description = "local metrics port for each librarian instance"
}

variable "grafana_port" {
  description = "port for Grafana service"
}

variable "prometheus_port" {
  description = "port for Prometheus service"
}

module "{{ .ClusterName }}" {
  source = "{{ .LocalModulePath }}"
  credentials_file = "${var.credentials_file}"
  gcs_clusters_bucket = "{{ .Bucket }}"
  cluster_name = "{{ .ClusterName }}"
  gcp_project = "{{ .GCPProject }}"
  num_librarians = "${var.num_librarians}"
  num_cluster_nodes = "${var.num_cluster_nodes}"
  gce_node_machine_type = "${var.cluster_node_machine_type}"
}
