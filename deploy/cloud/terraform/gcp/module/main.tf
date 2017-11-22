
provider "google" {
  project     = "${var.gcp_project}"
  region = "${var.gce_node_region}"
  credentials = "${file(var.credentials_file)}"
}

resource "google_container_cluster" "libri" {
  description = "libri cluster"
  name = "${var.cluster_name}"
  zone = "${var.gce_node_zone}"
  initial_node_count = "${var.num_cluster_nodes}"
  project = "${var.gcp_project}"

  master_auth {
    username = "admin"
    password = "demetrius"
  }

  node_config {
    machine_type = "${var.gce_node_machine_type}"
    disk_size_gb = "${var.node_disk_size_gb}"
  }

  provisioner "local-exec" {
    command = "gcloud container clusters get-credentials ${var.cluster_name} --zone ${var.gce_node_zone}"
  }
}

resource "google_compute_disk" "data-librarians" {
  count = "${var.num_librarians}"
  name = "data-librarians-${count.index}"
  type = "${var.librarian_disk_type}"
  zone = "${var.gce_node_zone}"
  size = "${var.librarian_disk_size_gb}"
}

resource "google_compute_firewall" "default" {
  description = "opens up ports for libri cluster communication to the outside world"
  name = "${var.cluster_name}"
  network = "default"

  allow {
    protocol = "tcp"
    ports = [
      "${var.min_libri_port}-${var.min_libri_port + var.num_librarians - 1}",
      "30300",
      "30090",
    ]
  }

  source_ranges = ["0.0.0.0/0"]
}
