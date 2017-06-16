# use stretch (debian) b/c `go test -race` requires glibc, which isn't in the alpine variant
FROM golang:1.8.3-stretch

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev cmake zip unzip tar gzip \
    vim emacs bash-completion

# install RocksDB
ENV ROCKSDB_VERSION '5.4.5'
RUN wget https://github.com/facebook/rocksdb/archive/v${ROCKSDB_VERSION}.zip -O /tmp/rocksdb-${ROCKSDB_VERSION}.zip && \
    unzip /tmp/rocksdb-${ROCKSDB_VERSION}.zip -d /tmp && \
    cd /tmp/rocksdb-${ROCKSDB_VERSION} && \
    make static_lib && \
    cd /tmp/rocksdb-${ROCKSDB_VERSION} && \
    make install && \
    rm /tmp/rocksdb-${ROCKSDB_VERSION}.zip && \
    rm -r /tmp/rocksdb-${ROCKSDB_VERSION}

# build & install gorocksdb
RUN CGO_CFLAGS="-I/usr/local/include/rocksdb" \
    CGO_LDFLAGS="-L/usr/local/lib/librocksdb.a -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" \
    go get github.com/tecbot/gorocksdb

# install Docker
ENV DOCKER_VERSION '17.03.0-ce'
RUN wget https://get.docker.com/builds/Linux/x86_64/docker-${DOCKER_VERSION}.tgz -O /tmp/docker-${DOCKER_VERSION}.tgz && \
    tar xzf /tmp/docker-${DOCKER_VERSION}.tgz -C /tmp && \
    mv /tmp/docker/* /usr/bin && \
    rm /tmp/docker-${DOCKER_VERSION}.tgz && \
    rm -r /tmp/docker

# set lang as UTF-8
RUN apt-get install -y locales && rm -rf /var/lib/apt/lists/* && \
    localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8
ENV LANG en_US.utf8

RUN mkdir -p "${GOPATH}/src/github.com/drausin/libri"
WORKDIR "${GOPATH}/src/github.com/drausin/libri"

ENTRYPOINT ["/bin/bash"]