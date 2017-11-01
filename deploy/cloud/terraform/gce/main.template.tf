
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

module "{{ .ClusterName }}" {
  source = "./module"  # TODO (drausin) add script to publish module to Terraform
  credentials_file = "${var.credentials_file}"
  gcs_clusters_bucket = "{{ .Bucket }}"
  cluster_name = "{{ .ClusterName }}"
  gcp_project = "{{ .GCPProject }}"
}
