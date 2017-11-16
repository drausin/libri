## Libri cluster deployment

Deploying a cluster involves creating infrastructure via Terraform and deploying services
and other things onto it via Kubernetes.

The components of a libri cluster are
- libri-headless: headless `ClusterIP` service for internal DNS resolution among librarians
- librarians-[0,...,N-1]: `NodePort` services for each of the librarians, making them accessible
to outside authors
- librarians: `StatefulSet` of N librarians
- data-librarians-[0,...,N-1]: a `PersistentVolume` and `PersistentVolumeClaim` for each librarian
- Prometheus service and deployment for monitoring
- Grafana service and deployment for dashboards


If you want to just try out the Kubernetes part first without creating GCP infrastucture,
you can run it via minikube (currently tested with v0.22)

#### Initializing

Initialize the cluster with

    go run cluster.go init \
        --outDir /path/to/clusters
        --clusterName my-test-cluster \
        --bucket my-bucket-name \
        --gcpProject my-gcp-project \

where `/path/to/clusters/dir` is the directory to create the cluster subdirectory in.

The `terraform.tfvars` file created in the cluster directory has settings (like number of
librarians) that can you can change if you want, though the default should be reasonable
enough to start.


#### Planning

To see what would be created upon spinning up a cluster called `my-cluster`, use

    go run cluster.go plan -c /path/to/clusters/my-cluster

You should first see Terraform plans and then planned dry run Kubernetes resources. If creating
cluster on minikube add `--minikube`

    go run cluster.go plan -c /path/to/clusters/my-cluster --minikube


#### Applying

Create the cluster and resources with

    go run cluster.go apply -c /path/to/clusters/my-cluster

which will first create the Terraform infrastructure and the the Kubernetes resources. If using
minikube, you can add the `--notf` as with the plan `command`.

    go run cluster.go apply -c /path/to/clusters/my-cluster --minikube

You should see the the resources being created (Terraform resources can take up to 5 minutes). See
the created pods with

    $ kubectl get pods -o wide
    NAME                          READY     STATUS    RESTARTS   AGE       IP               NODE
    grafana-1885146873-rh24j      1/1       Running   0          10m       172.17.0.6       minikube
    librarians-0                  1/1       Running   0          10m       172.17.0.3       minikube
    librarians-1                  1/1       Running   0          10m       172.17.0.7       minikube
    librarians-2                  1/1       Running   0          10m       172.17.0.8       minikube
    librarians-3                  1/1       Running   0          10m       172.17.0.9       minikube
    node-exporter-9c6jn           1/1       Running   0          10m       192.168.99.100   minikube
    prometheus-1589647967-0c0bn   1/1       Running   0          10m       172.17.0.5       minikube

and the created services with

    $ kubectl get services -o wide
    NAME           CLUSTER-IP   EXTERNAL-IP   PORT(S)           AGE       SELECTOR
    grafana        10.0.0.3     <nodes>       3000:30300/TCP    4m        app=grafana
    kubernetes     10.0.0.1     <none>        443/TCP           1d        <none>
    librarians-0   10.0.0.6     <nodes>       20100:30100/TCP   4m        hostname=librarians-0
    librarians-1   10.0.0.138   <nodes>       20100:30101/TCP   4m        hostname=librarians-1
    librarians-2   10.0.0.27    <nodes>       20100:30102/TCP   4m        hostname=librarians-2
    librarians-3   10.0.0.52    <nodes>       20100:30103/TCP   4m        hostname=librarians-3
    libri          None         <none>        20100/TCP         4m        app=libri
    prometheus     10.0.0.133   <nodes>       9090:30090/TCP    4m        app=prometheus


#### Testing

If using a local cluster, get the external address for one of the services

    $ minikube service librarians-0 --url
    http://192.168.99.100:30100

If using a GCE cluster, get an external address from one of the nodes

    $ gcloud compute instances list
    NAME                                      ZONE        MACHINE_TYPE   PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP      STATUS
    gke-libri-dev-default-pool-5ee39584-schn  us-east1-b  n1-standard-1               10.142.0.4   104.196.183.229  RUNNING
    gke-libri-dev-default-pool-5ee39584-tljc  us-east1-b  n1-standard-1               10.142.0.3   35.185.100.233   RUNNING
    gke-libri-dev-default-pool-5ee39584-w5vw  us-east1-b  n1-standard-1               10.142.0.2   35.196.233.112   RUNNING

    $ kubectl get pods -o wide
    NAME                          READY     STATUS    RESTARTS   AGE       IP           NODE
    grafana-1885146873-hz0ws      1/1       Running   0          8m        10.24.1.6    gke-libri-dev-default-pool-5ee39584-schn
    librarians-0                  1/1       Running   0          4m        10.24.0.6    gke-libri-dev-default-pool-5ee39584-w5vw
    librarians-1                  1/1       Running   1          4m        10.24.2.4    gke-libri-dev-default-pool-5ee39584-tljc
    librarians-2                  1/1       Running   0          3m        10.24.1.7    gke-libri-dev-default-pool-5ee39584-schn
    librarians-3                  1/1       Running   0          3m        10.24.0.7    gke-libri-dev-default-pool-5ee39584-w5vw
    node-exporter-pnbd5           1/1       Running   0          8m        10.142.0.2   gke-libri-dev-default-pool-5ee39584-w5vw
    node-exporter-r7slh           1/1       Running   0          8m        10.142.0.4   gke-libri-dev-default-pool-5ee39584-schn
    node-exporter-zwr2p           1/1       Running   0          8m        10.142.0.3   gke-libri-dev-default-pool-5ee39584-tljc
    prometheus-1589647967-rj06c   1/1       Running   0          8m        10.24.1.5    gke-libri-dev-default-pool-5ee39584-schn

For now, you need to visually "join" the pods to the instances to get the IP:Port combinations for
the librarians; for the above, they are
- librarians-0: 35.196.233.112:30100
- librarians-1: 35.185.100.233:30101
- librarians-2: 104.196.183.229:30102
- librarians-3: 35.196.233.112:30103

For convenience (and speed), you can run testing commands from an ephemeral container. Test the
health of a librarian with

    $ librarian_addrs='192.168.99.100:30100,192.168.99.100:30101,192.168.99.100:30102'
    $ docker run --rm daedalus2718/libri:snapshot test health -a "${librarian_addrs}"

Test uploading/downloading entries from the cluster with

    $ docker run --rm daedalus2718/libri:snapshot test io -a "${librarian_addrs}"

If you get timeout issues (especially with remote GCP cluster), try bumping the timeout up to 20 seconds with

    $ docker run --rm daedalus2718/libri:snapshot test io -a "${librarian_addrs}" --timeout 20


#### Monitoring

If using minikube, get the Grafana service address

    $ minikube service grafana --url
    http://192.168.99.100:30300

If using GCP, use the the same visual "join" as with librarians above to see that the NodePort public address
for the Grafana service is `104.196.183.229:30300`.


#### Updating

When you want to update the cluster (e.g., add a librarian or a node), just change the appropriate property in
`terraform.tfvars` and re-apply

    ./libri-cluster.sh apply /path/to/clusters/my-cluster

or, when using minikube,

    ./libri-cluster.sh apply /path/to/clusters/my-cluster --minikube

If the change you've applied involves the Prometheus or Grafana configmaps, you have to bounce the service manually
(since configmaps aren't a formal Kubernetes resources) to pick up the new config. Do this by just deleting the pod
and letting Kubernetes recreate it

    $ kubectl get pods
    NAME                          READY     STATUS    RESTARTS   AGE
    grafana-1885146873-rh24j      1/1       Running   0          1h
    librarians-0                  1/1       Running   0          1h
    librarians-1                  1/1       Running   0          1h
    librarians-2                  1/1       Running   0          1h
    librarians-3                  1/1       Running   0          1h
    node-exporter-9c6jn           1/1       Running   0          1h
    prometheus-1589647967-0c0bn   1/1       Running   0          1h

    $ kubectl delete pod grafana-1885146873-rh24j

    $ kubectl get pods
    NAME                          READY     STATUS        RESTARTS   AGE
    grafana-1885146873-h66zj      1/1       Running       0          12m
    grafana-1885146873-rh24j      0/1       Terminating   0          1h
    librarians-0                  1/1       Running       0          1h
    librarians-1                  1/1       Running       0          1h
    librarians-2                  1/1       Running       0          1h
    librarians-3                  1/1       Running       0          1h
    node-exporter-9c6jn           1/1       Running       0          1h
    prometheus-1589647967-0c0bn   1/1       Running       0          1h

    $ kubectl get pods
    NAME                          READY     STATUS    RESTARTS   AGE
    grafana-1885146873-h66zj      1/1       Running   0          13m
    librarians-0                  1/1       Running   0          1h
    librarians-1                  1/1       Running   0          1h
    librarians-2                  1/1       Running   0          1h
    librarians-3                  1/1       Running   0          1h
    node-exporter-9c6jn           1/1       Running   0          1h
    prometheus-1589647967-0c0bn   1/1       Running   0          1h


#### Destroying

When you're finished with a cluster, you have to destroy it manually, which you can do via

    $ kubectl delete -f /path/to/clusters/my-cluster/libri.yml

If you have Terraform infrastructure, you'll then use

    $ pushd /path/to/clusters/my-cluster
    $ terraform destroy
    $ popd