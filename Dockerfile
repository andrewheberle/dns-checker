FROM golang:1.25@sha256:931c889bca758a82fcbfcb1b6ed6ca1de30783e9e52e6093ad50060735cb99be AS builder

COPY . /build

RUN cd /build && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -ldflags '-w' ./cmd/dns-checker

FROM gcr.io/distroless/base-debian12:nonroot@sha256:107333192f6732e786f65df4df77f1d8bfb500289aad09540e43e0f7b6a2b816

LABEL org.opencontainers.image.description="DNS checker sidecar container"

COPY --from=builder /build/dns-checker /app/dns-checker

ENV DNS_LISTEN=":8080"

EXPOSE 8080

ENTRYPOINT [ "/app/dns-checker" ]
