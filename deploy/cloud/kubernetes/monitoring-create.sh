#!/usr/bin/env bash

set -eou pipefail

kubectl create configmap prometheus-config --from-file=config/prometheus
kubectl create configmap grafana-config --from-file=config/grafana

kubectl create -f monitoring.yml
