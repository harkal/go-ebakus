# Build Ebakus in a stock Go builder container
FROM golang:1.11-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git

# TARGET={fullnode|miningnode}
ENV TARGET=fullnode

RUN mkdir -p $GOPATH/src/github.com/ebakus
COPY ./github.com/harkal/ebakusdb $GOPATH/src/github.com/harkal/ebakusdb
COPY ./github.com/ebakus/node $GOPATH/src/github.com/ebakus/node
WORKDIR /go/src/github.com/ebakus/node
RUN go get -d -v ./cmd/ebakus
RUN go install -v ./cmd/ebakus

# Pull Ebakus into a second stage deploy alpine container
FROM alpine:latest

RUN mkdir /opt/shared
VOLUME /opt/shared

WORKDIR /opt/shared
COPY ./github.com/ebakus/node/ebakus/env/ebakus-testnet/shared_data ./shared_data

RUN mkdir -p /opt/ebakus

WORKDIR /opt/ebakus

RUN apk add --no-cache ca-certificates bash
COPY --from=builder /go/bin/ebakus .

COPY ./github.com/ebakus/node/ebakus/env/ebakus-testnet/run.sh .
RUN chmod a+x ./run.sh

EXPOSE 8545 8546 30401 30403 30404 6060

ENV NODE_NAME=""
ENV MINERTHREADS=1

ENTRYPOINT ["bash", "run.sh"]
CMD []
