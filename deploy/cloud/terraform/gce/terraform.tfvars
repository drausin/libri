# GCP credentials
# credentials_file = "/path/to/local-creds.json"

# librarians
num_librarians = 4
librarian_libri_version = "snapshot"
librarian_disk_size_gb = 10
librarian_cpu_limit = "250m"
librarian_ram_limit = "300M"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090

# Kubernetes cluster
num_cluster_nodes = 2
cluster_node_machine_type = "n1-standard-1"
