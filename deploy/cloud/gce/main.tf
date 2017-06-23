provider "google" {
  project     = "${var.gce_project}"
  region = "${var.gce_region}"
}

resource "google_container_cluster" "primary" {
  name = "libri"
  zone = "${var.gce_zone}"
  initial_node_count = "${var.num_instances}"
  machine_type = "${var.gce_machine_type}"
  project = "libri-170711"
  tags = ["libri"]

  master_auth {
    password = "???"
    username = "???"
  }

  node_config {
    machine_type = "${var.gce_machine_type}}"
    disk_size_gb = "${var.gce_disk_size_gb}"
    image_type = "${var.gce_image}"
  }
}


