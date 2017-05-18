GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/wadey/gocovmerge \
	 github.com/mattn/goveralls
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
ALL_PKGS=$(shell go list ./... | sed -r 's|github.com/drausin/libri/||g' | sort)
GIT_STATUS_PKGS=$(shell git status --porcelain | grep -e '\.go$$' | sed -r 's|^...(.+)/[^/]+\.go$$|\1|' | sort | uniq)
CHANGED_PKGS=$(shell echo $(ALL_PKGS) $(GIT_STATUS_PKGS) | tr " " "\n" | sort | uniq -d)

acceptance:
	@echo "--> Running acceptance tests"
	@go test -tags acceptance -v github.com/drausin/libri/libri/acceptance 2>&1 | tee acceptance.log

build:
	@echo "--> Running go build"
	@go build ./...

build-static:
	@echo "--> Running go build for static binary"
	@./scripts/build-static deploy/bin/libri

docker-image:
	@echo "--> Building docker image"
	@docker build --rm=false -t daedalus2718/libri:latest deploy

fix:
	@echo "--> Running goimports"
	@find . -name *.go | xargs goimports -l -w
	@echo "--> Running go fmt"
	@go fmt ./...

lint:
	@echo "--> Running gometalinter"
	@gometalinter ./... --config=.gometalinter.json --deadline=10m

lint-diff:
	@echo "--> Running gometalinter on packages with uncommitted changes"
	@echo $(CHANGED_PKGS) | tr " " "\n"
	@echo $(CHANGED_PKGS) | xargs gometalinter --config=.gometalinter.json --deadline=10m

lint-optional:
	@echo "--> Running gometalinter with optional linters"
	@gometalinter ./... --config=.gometalinter.optional.json --deadline=240s

proto:
	@echo "--> Running protoc"
	@protoc ./libri/author/keychain/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/common/ecid/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/librarian/api/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/common/storage/*.proto --go_out=plugins=grpc:.

test-cover:
	@echo "--> Running go test with coverage"
	@./scripts/test-cover

test:
	@echo "--> Running go test"
	@go test -race ./... --cover

tools:
	go get -u $(GOTOOLS)
	gometalinter --install

