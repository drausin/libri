# Libri infrastructure using Terraform

This directory contains the [Terraform](https://www.terraform.io) configuration for a infrastructure 
to suppose a simple libri cluster, including
- Google Container Engine cluster of 3x n1-standard-1 machines
- 3x persistent disks, one for each librarian
- firewall rule to open libri ports

#### Prerequisists

You'll need the following on your machine before getting started
- `gcloud` command line tool (see [installation instructions](https://cloud.google.com/sdk/downloads))
- Terraform client (see [the docs](https://www.terraform.io/intro/getting-started/install.html) 
or just used `brew install terraform`) 

You will also want to authenticate your machine with Google Cloud Platform (GCP)

    gcloud auth application-default login

You'll want to either create a new GCP project or use an existing one to use for this 
infrastructure. Store it in an environment variable that Terraform can use
 
    export TF_VAR_gcp_project='libri-XYZ'  # obviously, substitute your real project here
    
#### Creating

Run

    $ cd deploy/cloud/gce  # if not already there
    $ terraform plan

to see what Terraform plans to do. You should see something like

    + google_compute_disk.data-librarians-0
        ...
    
    + google_compute_disk.data-librarians-1
        ...
    
    + google_compute_disk.data-librarians-2
        ...
    
    + google_compute_firewall.default
        ...
    
    + google_container_cluster.libri
        ...
        
    Plan: 5 to add, 0 to change, 0 to destroy.
    
Assuming that looks good, create that infra with 

    # terraform apply
    
It may take up to 5 minutes to create the `google_container_cluster`.


#### Creating

List the GCE instances you just created

    $ gcloud compute instances list
    NAME                                  ZONE        MACHINE_TYPE   PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP     STATUS
    gke-libri-default-pool-9603795d-7pw5  us-east1-b  n1-standard-1               10.142.0.2   104.196.202.10  RUNNING
    gke-libri-default-pool-9603795d-cdb7  us-east1-b  n1-standard-1               10.142.0.4   104.196.30.174  RUNNING
    gke-libri-default-pool-9603795d-j1p4  us-east1-b  n1-standard-1               10.142.0.3   104.196.45.180  RUNNING
    
Assuming you're using Kubernetes to deploy the libri cluster, you might want to set up a 
`kubectl proxy` from your machine to the master Kubernetes node running in the cluster. Set this via
 
    kubectl proxy
    
which uses the Google default credentials stored when you created the cluster. Now `kubectl` 
commands run on your local machine will be proxied to apply to the remote cluster.
 
See libri's [Kubernetes README](../kubernetes/README.md) for creating the libri peers on this custer. 

#### Destroying

When you're finished with the cluster, destroy it with

    $ terraform destroy




    
