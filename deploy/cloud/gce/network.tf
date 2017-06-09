resource "google_compute_firewall" "default" {
  name = "libri-firewall"
  network = "default"

  allow {
    protocol = "tcp"
    ports = ["${var.sql_port}", "${var.http_port}", 9001 ]  # TODO (port range?)
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags = ["libri"]
}

