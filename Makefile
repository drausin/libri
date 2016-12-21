PACKAGES=$(shell go list ./... | grep -v '/vendor/')
GOTOOLS= github.com/axw/gocov/gocov \
	 gopkg.in/matm/v1/gocov-html
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr

# all builds binaries for all targets
all: build imports format vet test

build:
	@echo "--> Running go build"
	@go build $(PACKAGES)

cov:
	@gocov test $(PACKAGES) | gocov-html > /tmp/coverage.html
	@open /tmp/coverage.html

test:
	@echo "--> Running go test"
	@go test $(PACKAGES) --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

imports:
	@echo "--> Running goimports"
	@find . -name *.go | xargs goimports -l -w

vet:
	@echo "--> Running go tool vet $(VETARGS) ."
	@go list ./... \
		| grep -v /vendor/ \
		| cut -d '/' -f 4- \
		| xargs -n1 \
		go tool vet $(VETARGS) ;\

tools:
	go get -u -v $(GOTOOLS)

.PHONY: all build cov test format imports vet tools

