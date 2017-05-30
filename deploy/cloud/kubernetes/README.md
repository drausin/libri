# Kubernetes libri cluster

A Kubernetes config for a libri cluster is given in [libri.yml](libri.yml). The components are
- libri-headless: headless `ClusterIP` service for internal DNS resolution among librarians
- librarians-[0,...,N-1]: `NodePort` services for each of the librarians, making them accessible 
to outside authors
- librarians: `StatefulSet` of N librarians


#### Generating the config

Because the [libri.yml](libri.yml) has a separate service for each librarian, it is auto-generated
from [libri.template.yml](libri.template.yml), which is where edits should be made. Use 
[gen.go](gen.go) to generate from the template after editing
 
    $ cd libri/deploy/cloud/kubernetes
    $ go run gen.go -n 3  # generate a 3-librarian cluster
    wrote config to libri.yml

See the help (`-h`) in `gen.go` for all options. 


#### Starting the cluster

Start the cluster with 
    
    $ kubectl create -f libri.yml
    service "libri-headless" created
    service "librarians-0" created
    service "librarians-1" created
    service "librarians-2" created
    statefulset "librarians" created

and examine the pods with

    kubectl get pods -o wide --show-labels
    NAME           READY     STATUS    RESTARTS   AGE       IP           NODE       LABELS
    librarians-0   1/1       Running   0          1m        172.17.0.6   minikube   app=libri,hostname=librarians-0
    librarians-1   1/1       Running   0          1m        172.17.0.7   minikube   app=libri,hostname=librarians-1
    librarians-2   1/1       Running   0          1m        172.17.0.8   minikube   app=libri,hostname=librarians-2

Get the external address for one of the services

    $ minikube service librarians-0 --url
    http://192.168.99.100:30100


#### Testing the cluster

For convenience (and speed), you can run testing commands from an ephemeral container. Test the 
health of a librarian with

    $ docker run --rm daedalus2718/libri test health -a '192.168.99.100:30100'

Test uploading/downloading entries from the cluster with

    $ docker run --rm daedalus2718/libri test io -a '192.168.99.100:30100'


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