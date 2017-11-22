cluster_host = "gcp"

# GCP credentials
# credentials_file = "/path/to/local-creds.json"

# librarians
num_librarians = 8
librarian_libri_version = "snapshot"
librarian_disk_size_gb = 10
librarian_cpu_limit = "100m"
librarian_ram_limit = "1G"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090

# Kubernetes cluster
num_cluster_nodes = 2
cluster_node_machine_type = "n1-highmem-2"  # 2 CPUs, 6.5 GB RAM
