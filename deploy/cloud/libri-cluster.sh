#!/usr/bin/env bash

set -eou pipefail

usage() {
    echo "Operate a libri cluster"
    echo
    echo "Usage:"
    echo "  ./libri-cluster.sh [init|plan|apply] [flags]"
    echo
    echo "other binary dependencies: go, kubectl, terraform"
    echo
}

init_usage() {
    echo "Initialize a new libri cluster"
    echo
    echo "Usage:"
    echo
    echo "  ./libri-cluster.sh init [outDir]"
    echo
    echo "where [outDir] is the directory to create new cluster subdirectory in"
    echo
}

plan_usage() {
    echo "Plan changes to a libri cluster"
    echo
    echo "Usage:"
    echo
    echo "  ./libri-cluster.sh plan [clusterDir] [flags]"
    echo
    echo "where [clusterDir] is the directory created during initialization."
    echo
    echo "optional flags:"
    echo "  --notf      skip Terraform planning"
    echo "  --nokube    skip Kubernetes planning"
    echo "  --minikube  use minikube instead of Terraform infrastructure"
    echo
}

apply_usage() {
    echo "Apply changes to a libri cluster"
    echo
    echo "Usage:"
    echo
    echo "  ./libri-cluster.sh apply [clusterDir] [flags]"
    echo
    echo "where [clusterDir] is the directory created during initialization."
    echo
    echo "optional flags:"
    echo "  --notf      skip Terraform planning"
    echo "  --nokube    skip Kubernetes planning"
    echo "  --minikube  use minikube instead of Terraform infrastructure"
    echo
}

dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
subcommand="${1:-missing}"
n_args=$#
arg_2="${2:-}"
arg_3="${3:-}"
arg_4="${4:-}"

gen_kub_config() {
    # (Re-)Generate the Kubernetes configuration in the cluster directory using variables
    # defined in it's terraform.tfvars file.
    cluster_dir="${1}"
    minikube="${2}"
    infra_flag="--gce"
    if [[ "${minikube}" == "true" ]]; then
        infra_flag="--local"
    fi
    go run src/gen.go \
        "${infra_flag}" \
        -t '../kubernetes/libri.template.yml' \
        -o "${cluster_dir}/libri.yml" \
        -v "${cluster_dir}/terraform.tfvars"
}

gen_configmaps() {
    kubectl get configmap --no-headers | grep 'config' | awk '{print $1}' | xargs -I {} kubectl delete configmap {}
    kubectl create configmap prometheus-config --from-file=kubernetes/config/prometheus
    kubectl create configmap grafana-config --from-file=kubernetes/config/grafana
}

init() {
    if [[ ${n_args} -ne 2 ]]; then
        init_usage;
        exit 1
    fi
    out_dir="${arg_2}"

    echo "Welcome to libri cluster initialization. Please supply the following information:"
    echo
    echo -n "1/3) cluster name (kebab-case or snake_case; e.g., 'libri-dev'): "
    read cluster_name
    echo -n "2/3) GCP bucket for storage: "
    read bucket_name
    echo -n "3/3) GCP project to create infrastructure under: "
    read gcp_project
    echo
    echo "Consider manually adding your GCP credentials JSON file to the created terraform.tfvars."
    echo

    # generate Terraform config
    go run src/init.go -d "${out_dir}" -n "${cluster_name}" -b "${bucket_name}" -p "${gcp_project}"

    pushd "${out_dir}/${cluster_name}" > /dev/null 2>&1
    terraform init
    popd > /dev/null 2>&1

    echo
    echo "${cluster_name} cluster successfully initialized in ${out_dir}"
}

plan() {
    if [[ ${n_args} -lt 2 || ${n_args} -gt 3 ]]; then
        plan_usage;
        exit 1
    fi
    cluster_dir="${arg_2}"
    flag=${arg_3:-}
    minikube=false
    if [[ "${flag}" == "--minikube" ]]; then
        minikube=true
    fi

    if [[ "${flag}" != "--notf" && "${flag}" != "--minikube" ]]; then
        echo -e "Planning Terraform changes...\n"
        pushd "${cluster_dir}" > /dev/null 2>&1
        terraform plan
        popd > /dev/null 2>&1
        echo -e "\n"
    fi

    if [[ "${flag}" != "--nokube" ]]; then
        echo -e "Planning Kubernetes changes...\n"
        gen_kub_config "${cluster_dir}" "${minikube}"
        kubectl apply --dry-run -f "${cluster_dir}/libri.yml"
    fi
}

apply() {
    if [[ ${n_args} -lt 2 || ${n_args} -gt 3 ]]; then
        apply_usage;
        exit 1
    fi
    cluster_dir="${arg_2}"
    flag=${arg_3:-}
    minikube=false
    if [[ "${flag}" == "--minikube" ]]; then
        minikube=true
    fi

    if [[ "${flag}" != "--notf" && "${flag}" != "--minikube" ]]; then
        echo -e "Applying Terraform changes...\n"
        pushd "${cluster_dir}" > /dev/null 2>&1
        terraform apply -state current.tfstate
        popd > /dev/null 2>&1
        echo -e "\n"
    fi

    if [[ "${flag}" != "--nokube" ]]; then
        echo -e "Applying Kubernetes changes...\n"
        gen_kub_config "${cluster_dir}" "${minikube}"
        gen_configmaps
        kubectl apply -f "${cluster_dir}/libri.yml"
    fi
}

case "${subcommand}" in
    "init") init ;;
    "plan") plan ;;
    "apply") apply ;;
    * ) usage && exit 1
esac

