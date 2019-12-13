# Build Ebakus in a stock Go builder container
FROM golang:1.13-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git

ADD . /node
RUN cd /node && make ebakus

# Pull Ebakus into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /node/build/bin/ebakus /usr/local/bin/

EXPOSE 8545 8546 8547 30403 30403/udp
ENTRYPOINT ["ebakus"]
