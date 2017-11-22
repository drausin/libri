
variable "cluster_host" {
  description = "host of the cluster [gcp|minikube]"
}

variable "num_librarians" {
  description = "current number of librarian peers in cluster"
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
