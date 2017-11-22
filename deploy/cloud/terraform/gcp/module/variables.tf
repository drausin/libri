
############
# Required #
############

variable "credentials_file" {
  description = "GCP JSON credentials filepath"
}

variable "gcp_project" {
  description = "GCP project owning the cluster"
}

variable "gcs_clusters_bucket" {
  description = "GCS bucket for cluster state and backups"
}

variable "cluster_name" {
  description = "name of the libri cluster"
}


############
# Optional #
############

variable "gce_node_region" {
  default = "us-east1"
}

variable "gce_node_zone" {
  default = "us-east1-b"
}

variable "gce_node_network" {
  default = "default"
}

variable "num_cluster_nodes" {
  description = "current number of cluster nodes"
  default = 3
}

variable "gce_node_image_type" {
  default = "ubuntu-1604-xenial-v20170125"
}

variable "gce_node_machine_type" {
  default = "n1-standard-1"
}

variable "node_disk_size_gb" {
  description = "size (GB) of disk used by each cluster node"
  default = 25
}

variable "num_librarians" {
  description = "current number of librarian peers in cluster"
  default = 3
}

variable "librarian_disk_size_gb" {
  description = "size (GB) of persistant disk used by each librarian"
  default = 10
}

variable "librarian_disk_type" {
  description = "type of persistent disk used by each librarian"
  default = "pd-standard"
}

variable "min_libri_port" {
  default = 30100
}

# Name of the ssh key pair to use for GCE instances.
# The public key will be passed at instance creation, and the private
# key will be used by the local ssh client.
#
# The path is expanded to: ~/.ssh/<key_name>.pub
#
# If you use `gcloud compute ssh` or `gcloud compute copy-files`, you may want
# to leave this as "google_compute_engine" for convenience.
variable "key_name" {
  default = "google_compute_engine"
}
