output "cluster_node_addresses" {
  value = ["${google_container_cluster}.${var.cluster_name}.network_interface.0.access_config.0.assigned_nat_ip"]
}
