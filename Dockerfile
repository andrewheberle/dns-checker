FROM golang:1.24@sha256:d9db32125db0c3a680cfb7a1afcaefb89c898a075ec148fdc2f0f646cc2ed509 AS builder

COPY . /build

RUN cd /build && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -ldflags '-w' ./cmd/dns-checker

FROM gcr.io/distroless/base-debian12:nonroot@sha256:97d15218016debb9b6700a8c1c26893d3291a469852ace8d8f7d15b2f156920f

LABEL org.opencontainers.image.description="DNS checker sidecar container"

COPY --from=builder /build/dns-checker /app/dns-checker

ENV DNS_LISTEN=":8080"

EXPOSE 8080

ENTRYPOINT [ "/app/dns-checker" ]
