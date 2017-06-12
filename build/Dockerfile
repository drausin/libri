FROM golang:1.8.3-alpine

RUN apk --update add snappy zlib libbz2 \
    openssl wget zip git iptables procps build-base clang clang-libs
RUN apk add libpthread-stubs cmake

ENV ROCKSDB_VERSION '5.4.5'
RUN wget https://github.com/facebook/rocksdb/archive/v${ROCKSDB_VERSION}.zip -O /tmp/rocksdb-${ROCKSDB_VERSION}.zip && \
    unzip /tmp/rocksdb-${ROCKSDB_VERSION}.zip -d /tmp
RUN cd /tmp/rocksdb-${ROCKSDB_VERSION} && \
    make static_lib && \
    sudo make install

RUN CGO_CFLAGS="-I/usr/local/include/rocksdb" \
    CGO_LDFLAGS="-L/usr/local/lib/librocksdb.a -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" \
    go get github.com/tecbot/gorocksdb

#ENV DOCKER_VERSION '17.03.0-ce'
#RUN wget https://get.docker.com/builds/Linux/x86_64/docker-${DOCKER_VERSION}.tgz -O /tmp/docker-${DOCKER_VERSION}.tgz && \
#    tar -xz -C /tmp -f /tmp/docker-${DOCKER_VERSION}.tgz && \
#    mv /tmp/docker/* /usr/bin

ENTRYPOINT ["/bin/sh"]