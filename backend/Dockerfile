FROM golang:1.26.0-alpine AS builder

WORKDIR /build
COPY . .

ARG GOOS
ARG GOARCH
RUN CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -o secretdrop ./cmd/secretdrop/main.go

FROM alpine:latest AS certs
RUN apk add --update --no-cache ca-certificates

FROM busybox:latest
ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser

WORKDIR /app
RUN mkdir -p /app/db && chown appuser:appuser /app/db

USER appuser
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/secretdrop /app/secretdrop

ENTRYPOINT ["/app/secretdrop"]