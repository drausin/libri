PACKAGES=$(shell go list ./... | grep -v '/vendor/')
GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/axw/gocov/gocov \
	 gopkg.in/matm/v1/gocov-html
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr

# all builds binaries for all targets
all: build fix lint test

build:
	@echo "--> Running go build"
	@go build $(PACKAGES)

cov:
	@gocov test $(PACKAGES) | gocov-html > /tmp/coverage.html
	@open /tmp/coverage.html

test:
	@echo "--> Running go test"
	@go test $(PACKAGES) --cover

fix:
	@echo "--> Running goimports"
	@find . -name *.go | xargs goimports -l -w
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

lint:
	@echo "--> Running gometalinter"
	@gometalinter ./... --config=.gometalinter.json

tools:
	go get -u -v $(GOTOOLS)
	gometalinter --install

.PHONY: all build cov test fix lint tools

