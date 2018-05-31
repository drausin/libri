#!/usr/bin/env bash

set -eou pipefail

TEST_IMAGE="daedalus2718/libri:snapshot"
docker pull ${TEST_IMAGE}

pushd deploy/cloud
CLUSTERS_DIR="clusters"
mkdir -p ${CLUSTERS_DIR}
CLUSTER_DIR="${CLUSTERS_DIR}/rc"

go run cluster.go init minikube --clusterDir "${CLUSTER_DIR}" --clusterName "rc"

echo -e "\ncreated test cluster in ${CLUSTER_DIR}; ssh into minikube and pull Docker image if needed [press any key to continue]"
read

go run cluster.go apply --clusterDir ${CLUSTER_DIR}

echo -e '\nin separate shell, check that all pods came up successfully [press any key to continue]'
read

minikube_ip=$(minikube ip)
librarian_addrs="${minikube_ip}:30100,${minikube_ip}:30101,${minikube_ip}:30102,${minikube_ip}:30103"

docker run --rm ${TEST_IMAGE} test health -a "${librarian_addrs}"
docker run --rm ${TEST_IMAGE} test io -a "${librarian_addrs}"

echo -e '\neverything looks good! press any key to tear down the cluster and clean up'
read
kubectl delete -f ${CLUSTER_DIR}/libri.yml
rm -r ${CLUSTER_DIR}
popd
