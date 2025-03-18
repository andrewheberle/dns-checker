FROM golang:1.24@sha256:ceb568d0de81fbef40ce4fee77eab524a0a0a8536065c51866ad8c59b7a912cf AS builder

COPY . /build

RUN cd /build && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -ldflags '-w' ./cmd/dns-checker

FROM gcr.io/distroless/base-debian12:nonroot@sha256:23fa4a8575bc94e586b94fb9b1dbce8a6d219ed97f805369079eebab54c2cb23

LABEL org.opencontainers.image.description="DNS checker sidecar container"

COPY --from=builder /build/dns-checker /app/dns-checker

ENV DNS_LISTEN=":8080"

EXPOSE 8080

ENTRYPOINT [ "/app/dns-checker" ]
