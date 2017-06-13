FROM golang:1.8.3-alpine

RUN apk add --update \
    build-base linux-headers git cmake bash wget mercurial g++ autoconf cmake \
    zlib zlib-dev bzip2 bzip2-dev snappy snappy-dev lz4 lz4 jemalloc coreutils

# install gflags
ENV GFLAGS_VERSION '2.2.0'
RUN wget https://github.com/gflags/gflags/archive/v${GFLAGS_VERSION}.tar.gz -O /tmp/gflags-${GFLAGS_VERSION}.tar.gz && \
    tar xzf /tmp/gflags-${GFLAGS_VERSION}.tar.gz -C /tmp && \
    cd /tmp/gflags-${GFLAGS_VERSION} && \
    mkdir build && \
    cd build && \
    cmake -DBUILD_SHARED_LIBS=1 -DGFLAGS_INSTALL_SHARED_LIBS=1 .. && \
    make install && \
    cd /tmp && \
    rm /tmp/gflags-${GFLAGS_VERSION}.tar.gz && \
    rm -r /tmp/gflags-${GFLAGS_VERSION}


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

# compile gorocksdb
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

RUN mkdir -p "${GOPATH}/src/github.com/drausin/libri"
WORKDIR "${GOPATH}/src/github.com/drausin/libri"
ENTRYPOINT ["make"]