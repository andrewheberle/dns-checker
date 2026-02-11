FROM golang:1.25@sha256:85c0ab0b73087fda36bf8692efe2cf67c54a06d7ca3b49c489bbff98c9954d64 AS builder

COPY . /build

RUN cd /build && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -ldflags '-w' ./cmd/dns-checker

FROM gcr.io/distroless/base-debian12:nonroot@sha256:107333192f6732e786f65df4df77f1d8bfb500289aad09540e43e0f7b6a2b816

LABEL org.opencontainers.image.description="DNS checker sidecar container"

COPY --from=builder /build/dns-checker /app/dns-checker

ENV DNS_LISTEN=":8080"

EXPOSE 8080

ENTRYPOINT [ "/app/dns-checker" ]
