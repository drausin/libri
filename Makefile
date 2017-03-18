GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/wadey/gocovmerge \
	 github.com/mattn/goveralls
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
ALL_PKGS=$(shell go list ./... | sed -r 's|github.com/drausin/libri/||g' | sort)
GIT_STATUS_PKGS=$(shell git status --porcelain | grep -e '\.go$$' | sed -r 's|^...(.+)/[^/]+\.go$$|\1|' | sort | uniq)
CHANGED_PKGS=$(shell echo $(ALL_PKGS) $(GIT_STATUS_PKGS) | tr " " "\n" | sort | uniq -d)

# all builds binaries for all targets
all: build fix lint test acceptance

build:
	@echo "--> Running go build"
	@go build ./...

test-cover:
	@echo "--> Running go test with coverage"
	@./scripts/test-cover

test:
	@echo "--> Running go test"
	@go test -race ./... --cover

acceptance:
	@echo "--> Running acceptance tests"
	@go test -tags acceptance github.com/drausin/libri/libri/acceptance

fix:
	@echo "--> Running goimports"
	@find . -name *.go | xargs goimports -l -w
	@echo "--> Running go fmt"
	@go fmt ./...

lint:
	@echo "--> Running gometalinter"
	@gometalinter ./... --config=.gometalinter.json --deadline=8m

lint-diff:
	@echo "--> Running gometalinter on packages with uncommitted changes"
	@echo $(CHANGED_PKGS) | tr " " "\n"
	@echo $(CHANGED_PKGS) | xargs gometalinter --config=.gometalinter.json --deadline=6m

lint-optional:
	@echo "--> Running gometalinter with optional linters"
	@gometalinter ./... --config=.gometalinter.optional.json --deadline=240s

proto:
	@echo "--> Running protoc"
	@protoc ./libri/librarian/api/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/common/ecid/*.proto --go_out=plugins=grpc:.
	@protoc ./libri/librarian/server/storage/*.proto --go_out=plugins=grpc:.

tools:
	go get -u $(GOTOOLS)
	gometalinter --install

.PHONY: all build cov test acceptance fix lint tools
