cluster_host = "gcp"

# librarians
num_librarians = 8
librarian_libri_version = "0.2.0"
librarian_disk_size_gb = 10
librarian_disk_type = "pd-standard"
librarian_cpu_limit = "200m"
librarian_ram_limit = "2G"

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090

# Kubernetes cluster
num_cluster_nodes = 2
cluster_node_machine_type = "n1-highmem-2"  # 2 CPUs, 6.5 GB RAM
