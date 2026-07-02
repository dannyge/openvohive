# syntax=docker/dockerfile:1.4

FROM alpine:latest AS runner

ARG TARGETPLATFORM
ENV TZ=Asia/Shanghai

RUN apk add --no-cache alpine-conf ca-certificates su-exec tzdata libc6-compat && \
    /usr/sbin/setup-timezone -z Asia/Shanghai && \
    apk del alpine-conf && \
    rm -rf /var/cache/apk/*

WORKDIR /app

COPY ./app /app/server

ENTRYPOINT ["/app/server"]