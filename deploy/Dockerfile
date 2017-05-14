FROM alpine:3.5

ADD bin/libri /usr/local/bin  # add static binary
RUN mkdir /data
EXPOSE 20100

ENTRYPOINT ["libri"]
