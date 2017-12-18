
terraform {
  backend "gcs" {
    bucket  = "{{ .Bucket }}"
    path    = "{{ .ClusterName }}/terraform/current.tfstate"
    project = "{{ .GCPProject }}"
  }
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
  librarian_disk_type = "${var.librarian_disk_type}"
}
