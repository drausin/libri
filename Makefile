GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/axw/gocov/gocov \
	 gopkg.in/matm/v1/gocov-html
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr

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
	@go test ./... --cover

fix:
	@echo "--> Running goimports"
	@find . -name *.go | xargs goimports -l -w
	@echo "--> Running go fmt"
	@go fmt ./...

lint:
	@echo "--> Running gometalinter"
	@gometalinter ./... --config=.gometalinter.json --deadline=60s

proto:
	@echo "--> Running protoc"
	@find . -name '*.proto' -execdir protoc '{}' --go_out=plugins=grpc:. \;

tools:
	go get -u -v $(GOTOOLS)
	gometalinter --install

.PHONY: all build cov test fix lint tools

