GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/axw/gocov/gocov \
	 gopkg.in/matm/v1/gocov-html
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
ALL_PKGS=$(shell go list ./... | sed -r 's|github.com/drausin/libri/||g' | sort)
GIT_STATUS_PKGS=$(shell git status --porcelain | grep -e '\.go$$' | sed -r 's|^...(.+)/[^/]+\.go$$|\1|' | sort | uniq)
CHANGED_PKGS=$(shell echo $(ALL_PKGS) $(GIT_STATUS_PKGS) | tr " " "\n" | sort | uniq -d)

# all builds binaries for all targets
all: build fix lint test

build:
	@echo "--> Running go build"
	@go build ./...

cov:
	@echo "--> Running gocov test"
	@gocov test ./... | gocov-html > /tmp/coverage.html
	@open /tmp/coverage.html

test:
	@echo "--> Running go test"
	@go test -race ./... --cover

fix:
	@echo "--> Running goimports"
	@find . -name *.go | xargs goimports -l -w
	@echo "--> Running go fmt"
	@go fmt ./...

lint:
	@echo "--> Running gometalinter"
	@gometalinter ./... --config=.gometalinter.json --deadline=240s

lint-diff:
	@echo "--> Running gometalinter on packages with uncommitted changes on"
	@echo $(CHANGED_PKGS) | tr " " "\n"
	@echo $(CHANGED_PKGS) | xargs gometalinter --config=.gometalinter.json --deadline=240s

lint-optional:
	@echo "--> Running gometalinter with optional linters"
	@gometalinter ./... --config=.gometalinter.optional.json --deadline=240s

proto:
	@echo "--> Running protoc"
	@find . -name '*.proto' -execdir protoc '{}' --go_out=plugins=grpc:. \;

tools:
	go get -u $(GOTOOLS)
	gometalinter --install

.PHONY: all build cov test fix lint tools

