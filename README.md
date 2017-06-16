[![CircleCI](https://circleci.com/gh/drausin/libri/tree/develop.svg?style=shield)](https://circleci.com/gh/drausin/libri) [![codecov](https://codecov.io/gh/drausin/libri/branch/develop/graph/badge.svg)](https://codecov.io/gh/drausin/libri)


# libri

libri is a peer-to-peer distributed data storage network based on the [Kademila](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) protocol and approach. It will also offer
- end-to-end encryption
- notifications across the network for every storage event

#### status
libri is currently under active development and not yet ready for primetime

#### design
The peers of the network are called librarians. Each librarian exposes a set of simple endpoints, 
 descripted in [librarian.proto](https://github.com/drausin/libri/blob/develop/libri/librarian/api/librarian.proto) for getting and putting documents, described in [documents.proto](https://github.com/drausin/libri/blob/develop/libri/librarian/api/documents.proto).
 
Each librarian uses [RocksDB](https://github.com/facebook/rocksdb) for local storage.

The [acceptance tests](https://github.com/drausin/libri/blob/develop/libri/acceptance/librarian_test.go) and [librarian type](https://github.com/drausin/libri/blob/develop/libri/librarian/server/server.go) are good places from which to start exploring the codebase.

#### contributing


Run the tests with
```$bash
make test
```
and 
```$bash
make acceptance
```

Run the linters (can take a few minutes) via 
```$bash
make lint
```
or
```$bash
make lint-diff
```
if you just want to lint packages with unstaged changes (faster).


#### references
- [Kademila](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) protocol and approach
- libri is inspired by similar p2p distributed storage efforts Ethereum [Swarm](https://blog.ethereum.org/2016/12/15/swarm-alpha-public-pilot-basics-swarm/) and [Storj](https://storj.io/)
	- these two efforts also use variants of the Kademlia protocol
	- unlike these two, libri will not include any blockchain or contracts
