# Kubernetes libri cluster

A Kubernetes config template for a libri cluster is given in [libri.template.yml](libri.template.yml). 
The components are
- libri-headless: headless `ClusterIP` service for internal DNS resolution among librarians
- librarians-[0,...,N-1]: `NodePort` services for each of the librarians, making them accessible 
to outside authors
- librarians: `StatefulSet` of N librarians
- data-librarians-[0,...,N-1]: a `PersistentVolume` and `PersistentVolumeClaim` for each librarian 


#### Generating the config

Because we use a separate service for each librarian, it is auto-generated
from [libri.template.yml](libri.template.yml), which is where edits should be made. Use 
[gen.go](gen.go) to generate from the template after editing
 
To generate a `libri.yml` config for a local cluster running in minikube (good for testing most 
things)
 
    # generate a 3-librarian local cluster 
    $ go run gen.go -n 3 --local 
    wrote config to libri.yml
    
To generate a `libri.yml` config for a remote cluster running in Google Compute Engine

    # generate a 3-librarian remote GCE cluster 
    $ go run gen.go -n 3 --gce 
    wrote config to libri.yml

See the help (`-h`) in `gen.go` for all options. 


#### Starting the cluster

Start the cluster with 
    
    $ kubectl create -f libri.yml
    service "libri" created
    service "librarians-0" created
    persistentvolume "data-librarians-0" created
    persistentvolumeclaim "data-librarians-0" created
    service "librarians-1" created
    persistentvolume "data-librarians-1" created
    persistentvolumeclaim "data-librarians-1" created
    service "librarians-2" created
    persistentvolume "data-librarians-2" created
    persistentvolumeclaim "data-librarians-2" created
    statefulset "librarians" created

and examine the pods with

    kubectl get pods -o wide --show-labels
    NAME           READY     STATUS    RESTARTS   AGE       IP           NODE       LABELS
    librarians-0   1/1       Running   0          1m        172.17.0.6   minikube   app=libri,hostname=librarians-0
    librarians-1   1/1       Running   0          1m        172.17.0.7   minikube   app=libri,hostname=librarians-1
    librarians-2   1/1       Running   0          1m        172.17.0.8   minikube   app=libri,hostname=librarians-2

If using a local cluster, get the external address for one of the services

    $ minikube service librarians-0 --url
    http://192.168.99.100:30100

If using a GCE cluster, get an external address from one of the nodes

    $ gcloud compute instances list
    NAME                                  ZONE        MACHINE_TYPE   PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP     STATUS
    gke-libri-default-pool-c708ef91-1qsd  us-east1-b  n1-standard-1               10.142.0.3   35.185.116.166  RUNNING
    gke-libri-default-pool-c708ef91-584x  us-east1-b  n1-standard-1               10.142.0.4   104.196.147.89  RUNNING
    gke-libri-default-pool-c708ef91-r8g8  us-east1-b  n1-standard-1               10.142.0.2   35.185.88.75    RUNNING

    $ kubectl get pods -o wide --show-labels
    NAME           READY     STATUS    RESTARTS   AGE       IP          NODE                                   LABELS
    librarians-0   1/1       Running   0          8m        10.24.2.3   gke-libri-default-pool-c708ef91-584x   app=libri,hostname=librarians-0
    librarians-1   1/1       Running   0          8m        10.24.1.4   gke-libri-default-pool-c708ef91-1qsd   app=libri,hostname=librarians-1
    librarians-2   1/1       Running   0          7m        10.24.2.4   gke-libri-default-pool-c708ef91-584x   app=libri,hostname=librarians-2
    
For now, you need to visually "join" the pods to the instances to get the IP:Port combinations for 
the librarians; for the above, they are
- librarians-0: 104.196.147.89:30100
- librarians-1: 35.185.116.166:30101
- librarians-2: 104.196.147.89:30102
    
    
#### Testing the cluster

For convenience (and speed), you can run testing commands from an ephemeral container. Test the 
health of a librarian with

    $ docker run --rm daedalus2718/libri:latest test health -a '192.168.99.100:30100'

Test uploading/downloading entries from the cluster with

    $ docker run --rm daedalus2718/libri:latest test io -a '192.168.99.100:30100'


#### Monitoring the cluster

To start monitoring services (including Prometheus and Grafana), use

    $ ./monitoring-create.sh

If using a local cluster, you can get the URLs for these services via

    $ minikube service prometheus --url
    http://192.168.99.100:30090

    $ minikube service grafana --url
    http://192.168.99.100:30300


#### Terminating the cluster

Terminate the cluster with 

    $ kubectl delete -f libri.yml
    service "libri-headless" deleted
    service "librarians-0" deleted
    service "librarians-1" deleted
    service "librarians-2" deleted
    statefulset "librarians" deleted


#### Limitations

Until Kubernetes 1.7.0 is released and incorporated into minikube, we cannot take advantage of the 
`status.hostIP` downward API field (see [PR #42717](https://github.com/kubernetes/kubernetes/pull/42717)), 
which will allow librarian nodes to know what their public address is. Since the nodes in the 
cluster do not know their true external IPs, they are not discoverable (through bootstrapping) to 
the outside world. 

They are accessible, though, from author clients, which don't need to do any discovery and can 
connect to any librarian. This is how the `libri test` commands above connect. 