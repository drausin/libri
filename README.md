[![CircleCI](https://circleci.com/gh/drausin/libri/tree/develop.svg?style=shield)](https://circleci.com/gh/drausin/libri) [![codecov](https://codecov.io/gh/drausin/libri/branch/develop/graph/badge.svg)](https://codecov.io/gh/drausin/libri)


# Libri

Libri is a decentralized data storage network based on the 
[Kademila](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) protocol and 
approach. It also offers
- end-to-end encryption
- notifications across the network for every storage event

### Status
Libri is currently in alpha with a public testnet. 

### Try it out
Ensure you have Docker installed and then run
```bash
./libri/acceptance/local-demo.sh
```
to spin up a 4-node libri cluster, run some tests against it, and uploaded/download some sample data.

To try out (or join!) our public test network see [public testnet doc](libri/acceptance/public-testnet.md).

### Design

**Peers**
The peers of the network are called librarians. Each librarian exposes a set of simple endpoints, 
descripted in [librarian.proto](https://github.com/drausin/libri/blob/develop/libri/librarian/api/librarian.proto) 
for getting and putting documents, described in [documents.proto](https://github.com/drausin/libri/blob/develop/libri/librarian/api/documents.proto).
 
**Clients**
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

**Storage**
Each librarian and author uses [RocksDB](https://github.com/facebook/rocksdb) for local storage.

**Identity**
Author identity is managed through asymmetric 
[ECDH](https://en.wikipedia.org/wiki/Elliptic_curve_Diffie%E2%80%93Hellman) keys. When an author
is initialized it generates a cache of ECDH keys. These are always unique for each client, and the 
private key always stays on the local machine.

**Encryption**
When uploading a document, the author 
1) generates a new set of entry encryption keys,
2) uses it to encrypt the data contents into an [Entry](libri/librarian/api/documents.proto#L46) and 
 publishes that to the Libri network,
3) selects two of its own keys ECDH keys and uses their ECDH shared secret to generate a key 
 encryption key,
4) uses this key encryption key to encrypt the entry encryption key
5) uploads an [Envelope](libri/librarian/api/documents.proto#L26) containing the two public keys 
 and the entry encryption key ciphertext

When an author wants to share a document with another author, it just repeats steps 3-5 but with 
a public key of the author to send the document to. Only the Envelope is different (rather than
re-encrypting the entire Entry).

**Replication**
Documents are uploaded with a specified number of replicas. If peers storing those replicas drop out 
of the network, the other peers storing the remaining replicas take charge of storing additional 
copies to bring the replication factor up to a given level.

### Containers
Libri relies heavily on Docker containers, both for development and deployment. The development 
image ([daedalus2718/libri-build](https://hub.docker.com/r/daedalus2718/libri-build/)) is fairly large
(~1.5GB) because it contains all the binary dependencies needed for testing and development. The 
deployment image ([daedalus2718/libri](https://hub.docker.com/r/daedalus2718/libri/)) is fairly 
small (~90MB) because it contains only the things needed to run the `libri` command line binary.


### Contribute
See [CONTRIBUTING.md](CONTRIBUTING.md) for details.  Issues, suggestions, and pull requests always welcome.

Get in touch via `contact` AT `libri.io`.

### References
- [Kademila](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) protocol and approach
- libri is inspired by similar p2p distributed storage efforts Ethereum 
[Swarm](https://blog.ethereum.org/2016/12/15/swarm-alpha-public-pilot-basics-swarm/) and 
[Storj](https://storj.io/), among others
	- these two efforts also use variants of the Kademlia protocol
	- unlike these two, libri will not include any blockchain or contracts
