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
    echo "  --nokub     skip Kubernetes planning"
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
    echo "  --nokub     skip Kubernetes planning"
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
    go run src/gen.go \
        --gce \
        -t '../kubernetes/libri.template.yml' \
        -o "${cluster_dir}/libri.yml" \
        -v "${cluster_dir}/terraform.tfvars"
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
    skipflag=${arg_3:-}

    if [[ "${skipflag}" != "--notf" ]]; then
        echo -e "Planning Terraform changes...\n"
        pushd "${cluster_dir}" > /dev/null 2>&1
        terraform plan
        popd > /dev/null 2>&1
        echo -e "\n"
    fi

    if [[ "${skipflag}" != "--nokub" ]]; then
        echo -e "Planning Kubernetes changes...\n"
        gen_kub_config "${cluster_dir}"
        kubectl apply --dry-run -f "${cluster_dir}/libri.yml"
    fi
}

apply() {
    if [[ ${n_args} -lt 2 || ${n_args} -gt 3 ]]; then
        apply_usage;
        exit 1
    fi
    cluster_dir="${arg_2}"
    skipflag=${arg_3:-}

    if [[ "${skipflag}" != "--notf" ]]; then
        echo -e "Applying Terraform changes...\n"
        pushd "${cluster_dir}" > /dev/null 2>&1
        terraform apply
        popd > /dev/null 2>&1
        echo -e "\n"
    fi

    if [[ "${skipflag}" != "--nokub" ]]; then
        echo -e "Applying Kubernetes changes...\n"
        gen_kub_config "${cluster_dir}"
        kubectl apply -f "${cluster_dir}/libri.yml"
    fi
}

case "${subcommand}" in
    "init") init ;;
    "plan") plan ;;
    "apply") apply ;;
    * ) usage && exit 1
esac

