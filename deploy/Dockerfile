FROM alpine:3.5

ADD deploy/bin/libri /usr/local/bin
RUN mkdir /data
EXPOSE 20100

ENTRYPOINT ["libri"]
