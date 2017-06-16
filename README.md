[![CircleCI](https://circleci.com/gh/drausin/libri/tree/develop.svg?style=shield)](https://circleci.com/gh/drausin/libri) [![codecov](https://codecov.io/gh/drausin/libri/branch/develop/graph/badge.svg)](https://codecov.io/gh/drausin/libri)


# Libri

libri is a peer-to-peer distributed data storage network based on the 
[Kademila](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) protocol and 
approach. It will also offer
- end-to-end encryption
- notifications across the network for every storage event

#### Status
libri is currently under active development and not yet ready for primetime


#### Design

*Peers*
The peers of the network are called librarians. Each librarian exposes a set of simple endpoints, 
descripted in [librarian.proto](https://github.com/drausin/libri/blob/develop/libri/librarian/api/librarian.proto) 
for getting and putting documents, described in [documents.proto](https://github.com/drausin/libri/blob/develop/libri/librarian/api/documents.proto).
 
*Clients*
The clients of the network are called authors. Each other connects to one or more librarian peers
to upload/download documents and receive publications when others upload documents they are 
interested in.

This simple architecture looks something like
```
                       ┌───────────┐
                       │┌──────────┴┐
                       └┤ librarian │
                        └───────────┘
                              ▲
                              │
           ┌───────────┐      │    ┌───────────┐
           │┌──────────┴┐     │    │┌──────────┴┐
           └┤ librarian │◀────┴────▶┤ librarian │
            └───────────┘           └───────────┘
                  ▲                       ▲
                  │                       │        public libri network
─ ─ ─ ─ ─ ─ ─ ─ ─ ┼ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┼ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─
            ┌─────┴────┐                  │             private clients
            │          │                  │
            ▼          ▼                  ▼
       ┌────────┐ ┌────────┐         ┌────────┐
       │ author │ │ author │         │ author │
       └────────┘ └────────┘         └────────┘
```
Currently, we have only a Golang author implementation, but we expect to develop other 
implementations (e.g., Javascript) soon. 

*Storage*
Each librarian and author uses [RocksDB](https://github.com/facebook/rocksdb) for local storage.

#### Containers
Libri relies heavily on Docker containers, both for development and deployment. The development 
image ([daedalus2718/libri-build](https://hub.docker.com/r/daedalus2718/libri-build/)) is fairly large
(~1.5GB) because it contains all the binary dependencies needed for testing and development. The 
deployment image ([daedalus2718/libri](https://hub.docker.com/r/daedalus2718/libri/)) is fairly 
small (~90MB) because it contains only the things needed to run the `libri` command line binary.


#### Try it out
Ensure you have Docker installed and then run
```bash
./libri/acceptance/local-demo.sh
```
to spin up a 3-node libri cluster, run some tests against it, and uploaded/download some sample data.

#### Contribute
See [CONTRIBUTING.md](CONTRIBUTING.md) for details.  Issues, suggestions, and pull requests always welcome.

#### References
- [Kademila](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) protocol and approach
- libri is inspired by similar p2p distributed storage efforts Ethereum [Swarm](https://blog.ethereum.org/2016/12/15/swarm-alpha-public-pilot-basics-swarm/) and [Storj](https://storj.io/)
	- these two efforts also use variants of the Kademlia protocol
	- unlike these two, libri will not include any blockchain or contracts
