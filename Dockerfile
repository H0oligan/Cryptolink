FROM golang:1.26.10 AS builder

ARG EMBED_FRONTEND

WORKDIR /src

COPY go.mod /src/
COPY go.sum /src/

COPY . /src

RUN make build

ENTRYPOINT []

# Prepare production-ready image
FROM alpine:3.22

RUN apk add --no-cache tzdata libc6-compat
ENV TZ=UTC

COPY --from=builder /src/bin/oxygen /opt/

ENTRYPOINT ["/opt/oxygen"]
