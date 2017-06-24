# disks for PersistantVolumes of each librarian pod

resource "google_compute_disk" "data-librarians-0" {
  name = "data-librarians-0"
  type = "pd-standard"
  zone = "${var.gce_node_zone}"
  size = "${var.librarian_disk_size_gb}"
}

resource "google_compute_disk" "data-librarians-1" {
  name = "data-librarians-1"
  type = "pd-standard"
  zone = "${var.gce_node_zone}"
  size = "${var.librarian_disk_size_gb}"
}

resource "google_compute_disk" "data-librarians-2" {
  name = "data-librarians-2"
  type = "pd-standard"
  zone = "${var.gce_node_zone}"
  size = "${var.librarian_disk_size_gb}"
}
