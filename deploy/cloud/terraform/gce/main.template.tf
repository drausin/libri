
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

variable "num_cluster_nodes" {
  description = "current number of cluster nodes"
}

variable "gce_node_machine_type" {
  default = "n1-standard-1"
}

module "{{ .ClusterName }}" {
  source = "./module"  # TODO (drausin) add script to publish module to Terraform
  credentials_file = "${var.credentials_file}"
  gcs_clusters_bucket = "{{ .Bucket }}"
  cluster_name = "{{ .ClusterName }}"
  gcp_project = "{{ .GCPProject }}"
  num_librarians = "${var.num_librarians}"
  num_cluster_nodes = "${var.num_cluster_nodes}"
  gce_node_machine_type = "${var.gce_node_machine_type}"
}
