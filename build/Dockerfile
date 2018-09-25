# use stretch (debian) b/c `go test -race` requires glibc, which isn't in the alpine variant
FROM golang:1.11.0-stretch

# install Docker
ENV DOCKER_VERSION '17.03.0-ce'
RUN wget https://get.docker.com/builds/Linux/x86_64/docker-${DOCKER_VERSION}.tgz -O /tmp/docker-${DOCKER_VERSION}.tgz && \
    tar xzf /tmp/docker-${DOCKER_VERSION}.tgz -C /tmp && \
    mv /tmp/docker/* /usr/bin && \
    rm /tmp/docker-${DOCKER_VERSION}.tgz && \
    rm -r /tmp/docker

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev cmake zip unzip tar gzip \
    vim emacs bash-completion locales

# set lang as UTF-8
RUN rm -rf /var/lib/apt/lists/* && \
    localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8
ENV LANG en_US.utf8

# install RocksDB & gorocksdb
ADD *.sh /usr/local/bin/
RUN /usr/local/bin/install-rocksdb.sh && \
    /usr/local/bin/install-gorocksdb.sh

ENV GOPATH "/go"
RUN mkdir -p "${GOPATH}/src/github.com/drausin/libri"
WORKDIR "${GOPATH}/src/github.com/drausin/libri"

ENTRYPOINT ["/bin/bash"]