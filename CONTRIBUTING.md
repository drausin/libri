## Contributing

### Developing
Fork the project and clone to your development machine. Get the Golang dependencies onto your local
development machine via
```bash
make get-deps
```
We use [dep](https://github.com/golang/dep) for vendoring. You will also need Docker installed.

### Exploring

The [acceptance tests](libri/acceptance/librarian_test.go) and 
[librarian type](libri/librarian/server/server.go) are good places from which to start exploring 
the codebase. See the [libri librarian start](libri/cmd/start.go) and 
[libri author upload](libri/cmd/upload.go) commands for example the CLI entrypoints. 

### Testing
The simplest way to run the tests is from within a build container, which has all the required
binaries (e.g., RocksDB) already installed and linked. Our [CI](.circleci/config.yml) uses it.

*N.B., currently the local build container is a bit slower/more laggy than I would like. Improvement
suggestions are very welcome.*

The build container mounts
- `~/.go/src`, so your libri code and its dependencies are available
- `~/.bashrc`, so your build container shell is nice and familiar
- `~/.gitconfig`, so you can do all your favorite git things

To start it, run
```bash
./scripts/run-build-container.sh
```
which brings you into the build container. From there you can run most things you'd care about.
The most common `make` targets are
- `make test`: run all tests
- `make acceptance`: run the acceptance tests
- `make lint-diff`: lint the uncommitted changes
- `make lint`: lint the entire repo
- `make fix`: run `goimports` & `go fmt` on repo
Of course you can also run normal `go` tool commands or any other shell command you like.

You won't be able to run things requiring `docker run` (which you can't do from within a container), 
including
- `make demo` (or the underlying [local-demo.sh](libri/acceptance/local-demo.sh))
- starting a local Kubernetes cluster from `deloy/cloud/kubernetes/libri.yml` 

If you want to run tests locally (i.e., not in the build container), you'll have do the local 
installation (see below).

### Local OSX installation

This requires a tad more setup and obviously isn't as isolated as the build container, but it's 
faster since it's ultimately just your local machine.

First [install RocksDB](https://github.com/facebook/rocksdb/blob/master/INSTALL.md)
```$bash
brew install rocksdb
```
Then build the [gorocksdb](https://github.com/tecbot/gorocksdb) driver
```$bash
./build/install-gorocksdb.sh
```

