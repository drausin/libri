## Libri cluster in Google Cloud Platform

This Terraform module defines all the infrastructure required to run a libri
cluster on GCP, including

- Google Container Engine (GKE) cluster for running Kubernetes
- Persistent disks for each librarian in the cluster
- firewall rule to expoose librarians to public internet


### Usage

```
module "libri-staging" {
  source = "/path/to/local/module"

  # these are required
  credentials_file = "my-gcp-credentials.json"
  gcs_clusters_bucket = "my-libri-clusters"
  cluster_name = "libri-staging"
  gcp_project = "libri-12345"

  # these, among others, are optional and are commonly set in separate
  # properties.tfvars file
  num_librarians = 3
  num_cluster_nodes = 3
  gce_node_machine_type = "n1-standard-1"
}
```
