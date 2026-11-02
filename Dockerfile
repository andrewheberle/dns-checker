FROM cgr.dev/chainguard/static:latest@sha256:fc42af33e643f20d08baf03cddcc1c02ba57052dfb02edb0d79f3fc47bcea8d9
ARG TARGETPLATFORM
ENV DNS_LISTEN=":8080"
EXPOSE 8080
ENTRYPOINT [ "/usr/bin/dns-checker" ]
COPY $TARGETPLATFORM/dns-checker /usr/bin/
USER 65532