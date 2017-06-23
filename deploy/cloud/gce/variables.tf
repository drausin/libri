variable "gce_project" {
  default = "libri-170711"
}

variable "gce_node_region" {
  default = "us-east1"
}

variable "gce_node_zone" {
  description = "cluster node zone"
  default = "us-east1-b"
}

variable "gce_node_network" {
  description = "cluster node network"
  default = "default"
}

variable "gce_node_image_type" {
  description = "cluster node disk image"
  default = "ubuntu-1604-xenial-v20170125"
}

variable "gce_node_machine_type" {
  description = "cluster node machine type "
  default = "n1-standard-1"
}

variable "node_disk_size_gb" {
  description = "size (GB) of disk used by each cluster node"
  default = 25
}

variable "librarian_disk_size_gb" {
  description = "size (GB) of persistant disk used by each librarian"
  default = 10
}

variable "num_nodes" {
  description = "number of cluster nodes"
  default = 3
}

variable "min_libri_port" {
  default = 30100
}

variable "max_libri_port" {
  default = 30102
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

