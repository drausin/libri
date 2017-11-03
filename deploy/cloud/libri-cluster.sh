#!/usr/bin/env bash

set -eou pipefail

usage() {
    echo "Operate a libri cluster"
    echo
    echo "Usage:"
    echo "  libri-cluster [init|plan|apply] [flags]"
    echo
    echo "other binary dependencies: go, kubectl, terraform"
    echo
}

init_usage() {
    echo "Initialize a new libri cluster"
    echo
    echo "Usage:"
    echo
    echo "  libri cluster init [outDir]"
    echo
    echo "where [outDir] is the directory to create new cluster subdirectory in"
    echo ""
}

dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
subcommand="${1:-missing}"
n_args=$#
arg_2="${2:-}"

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

    # generate Terraform config
    go run lib/init.go -d "${out_dir}" -n "${cluster_name}" -b "${bucket_name}" -p "${gcp_project}"

    pushd "${out_dir}/${cluster_name}" > /dev/null 2>&1
    terraform init
    popd > /dev/null 2>&1

    echo
    echo "${cluster_name} cluster successfully initialized in ${out_dir}"
}


case "${subcommand}" in
    "init")
        init ;;
    * )
        usage
        exit 1
esac

