## Contributing

#### Developing
Fork the project and clone to your development machine. Get the Golang dependencies onto your local
development machine via
```bash
make get-dependencies
```
You will also need Docker installed.

#### Testing
The simplest way to run the tests is from within a build container, which has all the required
binaries (e.g., RocksDB) already installed and linked. The build container mounts
- `~/.go/src`, so your libri code and its dependencies are available
- `~/.bashrc`, so your build container shell is nice and familier
- `~/.gitconfig`, so you can do all your favorite git things
```bash
./scripts/run-build-container.sh
```
which brings you into the build container. From there you can run most things you'd care about
except those requiring `docker run` (which you can't do from within a container)
- `make demo`
- `./libri/acceptance/local-demo.sh` 

If you want to run these locally, you'll have to also do the local installation (see below).

The most common `make` targets are
- `make test`: run all tests
- `make acceptance`: run the acceptance tests
- `make lint-diff`: lint the uncommitted changes
- `make lint`: lint the entire repo

#### Local OSX installation

First [install RocksDB](https://github.com/facebook/rocksdb/blob/master/INSTALL.md).
Then build the [gorocksdb](https://github.com/tecbot/gorocksdb) driver.
```$bash
CGO_CFLAGS="-I/usr/local/include/rocksdb" \
CGO_LDFLAGS="-L/usr/local/opt/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4" \
  go get github.com/tecbot/gorocksdb
```

