#!/usr/bin/env bash -eou pipefail

docker run --rm -it \
	-h libri-build \
	-v ~/.go/src:/go/src \
	-v ~/.bashrc:/root/.bashrc \
	-v ~/.gitconfig:/root/.gitconfig \
	daedalus2718/libri-build:latest
