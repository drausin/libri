provider "google" {
  project     = "${var.gce_project}"
  region = "${var.gce_node_region}"
}

resource "google_container_cluster" "libri" {
  description = "libri cluster"
  name = "libri"
  zone = "${var.gce_node_zone}"
  initial_node_count = "${var.num_nodes}"
  project = "libri-170711"

  master_auth {
    username = "admin"
    password = "demetrius"
  }

  node_config {
    machine_type = "${var.gce_node_machine_type}"
    disk_size_gb = "${var.node_disk_size_gb}"
  }
}

resource "google_compute_firewall" "default" {
  description = "opens up ports for libri cluster communication to the outside world"
  name = "libri"
  network = "default"

  allow {
    protocol = "tcp"
    ports = ["${var.min_libri_port}-${var.max_libri_port}"]
  }

  source_ranges = ["0.0.0.0/0"]
}

output "instances" {
  value = "${join(",", google_container_cluster.libri.)}"
}
