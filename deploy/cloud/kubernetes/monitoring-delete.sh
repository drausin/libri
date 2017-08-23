#!/usr/bin/env bash

set -eou pipefail

kubectl delete -f monitoring.yml
kubectl delete configmap prometheus-config grafana-config
