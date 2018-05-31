cluster_host = "minikube"
cluster_admin_user = "admin"

# librarians
num_librarians = 4
librarian_libri_version = "snapshot"
librarian_cpu_limit = "50m"
librarian_ram_limit = "300M"
librarian_disk_size_gb = 1

librarian_public_port_start = 30100
librarian_local_port = 20100
librarian_local_metrics_port = 20200

# monitoring
grafana_port = 30300
prometheus_port = 30090
grafana_ram_limit = "100M"
prometheus_ram_limit = "250M"
grafana_cpu_limit = "250m"
prometheus_cpu_limit = "250m"
