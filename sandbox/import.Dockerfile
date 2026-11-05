FROM alpine:3.23
RUN apk add --no-cache git ca-certificates socat
COPY sandbox/import-entrypoint.sh /usr/local/bin/lcme-import
RUN sed -i 's/\r$//' /usr/local/bin/lcme-import && chmod 755 /usr/local/bin/lcme-import
ENV HOME=/tmp \
    GIT_CONFIG_NOSYSTEM=1 \
    GIT_CONFIG_SYSTEM=/dev/null \
    GIT_CONFIG_GLOBAL=/dev/null \
    GIT_TERMINAL_PROMPT=0 \
    GIT_ALLOW_PROTOCOL=https
USER 65534:65534
ENTRYPOINT ["/usr/local/bin/lcme-import"]
