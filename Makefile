SHELL=/bin/bash -eou pipefail
GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/wadey/gocovmerge
LIBRI_PKGS=$(shell go list ./... | grep -v /vendor/)
LIBRI_PKG_SUBDIRS=$(shell go list ./... | grep -v /vendor/ | sed -r 's|github.com/drausin/libri/||g' | sort)
GIT_STATUS_SUBDIRS=$(shell git status --porcelain | grep -e '\.go$$' | sed -r 's|^...(.+)/[^/]+\.go$$|\1|' | sort | uniq)
GIT_DIFF_SUBDIRS=$(shell git diff develop..HEAD --name-only | grep -e '\.go$$' | sed -r 's|^(.+)/[^/]+\.go$$|\1|' | sort | uniq)
GIT_STATUS_PKG_SUBDIRS=$(shell echo $(LIBRI_PKG_SUBDIRS) $(GIT_STATUS_SUBDIRS) | tr " " "\n" | sort | uniq -d)
GIT_DIFF_PKG_SUBDIRS=$(shell echo $(LIBRI_PKG_SUBDIRS) $(GIT_DIFF_SUBDIRS) | tr " " "\n" | sort | uniq -d)


.PHONY: bench build

acceptance:
	@echo "--> Running acceptance tests"
	@go test -tags acceptance -v github.com/drausin/libri/libri/acceptance 2>&1 | tee artifacts/acceptance.log

bench:
	@echo "--> Running benchmarks"
	@./scripts/run-author-benchmarks.sh

build:
	@echo "--> Running go build"
	@go build $(LIBRI_PKGS)

build-static:
	@echo "--> Running go build for static binary"
	@./scripts/build-static.sh deploy/bin/libri

demo:
	@echo "--> Running demo"
	@./libri/acceptance/local-demo.sh

docker-build-image:
	@docker build -t daedalus2718/libri-build:latest build

docker-image:
	@echo "--> Building docker image"
	@docker build --rm=false -t daedalus2718/libri:latest deploy

fix:
	@echo "--> Running goimports"
	@find . -name *.go | grep -v /vendor/ | xargs goimports -l -w

get-deps:
	@echo "--> Getting dependencies"
	@go get -u github.com/golang/dep/cmd/dep
	@dep ensure
	@go get -u -v $(GOTOOLS)
	@gometalinter --install

lint:
	@echo "--> Running gometalinter"
	@gometalinter $(LIBRI_PKG_SUBDIRS) --config=.gometalinter.json --deadline=10m

lint-diff:
	@echo "--> Running gometalinter on packages with uncommitted changes"
	@echo $(GIT_STATUS_PKG_SUBDIRS) | tr " " "\n"
	@echo $(GIT_STATUS_PKG_SUBDIRS) | xargs gometalinter --config=.gometalinter.json --deadline=10m

lint-slow:
	@echo "--> Running gometalinter slow linters"
	@gometalinter $(LIBRI_PKG_SUBDIRS) --config=.gometalinter.slow.json --deadline=30m

proto:
	@echo "--> Running protoc"
	@protoc ./libri/author/keychain/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/common/ecid/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/librarian/api/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/common/storage/*.proto --go_out=plugins=grpc:.

test-cover:
	@echo "--> Running go test with coverage"
	@./scripts/test-cover.sh

test:
	@echo "--> Running go test"
	@go test -race $(LIBRI_PKGS)

test-stress:
	@echo "--> Running stress tests"
	@./scripts/stress-test.sh



