provider "google" {
  region = "${var.gce_region}"
}

resource "google_compute_instance" "libri" {
  count = "${var.num_instances}"

  name = "libri-node-${count.index}"
  machine_type = "${var.gce_machine_type}"
  zone = "${var.gce_zone}"
  tags = ["libri"]

  disk {
    # TODO (persistent disks)
    image = "${var.gce_image}"
  }

  network_interface {
    network = "default"
    access_config {
        # Ephemeral
    }
  }

  metadata {
    sshKeys = "ubuntu:${file("~/.ssh/${var.key_name}.pub")}"
  }

  connection {
    user = "ubuntu"
    private_key = "${file(format("~/.ssh/%s", var.key_name))}"
  }

  service_account {
    scopes = ["https://www.googleapis.com/auth/compute.readonly"]
  }
}


