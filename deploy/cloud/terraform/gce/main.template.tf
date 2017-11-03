
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

# TODO (drausin) add after parameterizing in k8s template
# - pod ram & cpu limits
# - libri version

variable "num_cluster_nodes" {
  description = "current number of cluster nodes"
}

variable "cluster_node_machine_type" {
  description = "GCE cluster node machine type"
}

variable "librarian_public_port_start" {
  description = "public port for librarian-0 service"
  default = 30100
}

variable "librarian_local_port" {
  description = "local port for each librarian instance"
  default = 20100
}

variable "librarian_local_metrics_port" {
  description = "local metrics port for each librarian instance"
  default = 20200
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
