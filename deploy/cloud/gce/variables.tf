# GCE region to use.
variable "gce_region" {
  default = "us-east1"
}

# GCE zone to use.
variable "gce_zone" {
  default = "us-east1-b"
}

# GCE image name.
variable "gce_image" {
  default = "ubuntu-os-cloud/ubuntu-1604-xenial-v20170125"
}

# GCE machine type.
variable "gce_machine_type" {
  default = "n1-standard-1"
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

# Number of instances to start.
variable "num_instances" {
}

