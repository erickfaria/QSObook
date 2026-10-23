# Use a supported, security-patched compiler; go.mod retains its minimum version.
FROM --platform=$BUILDPLATFORM golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 AS build
ARG TARGETOS=linux
ARG TARGETARCH
ARG APP_VERSION=dev
ARG APP_REVISION=unknown
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOTELEMETRY=off
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -mod=readonly -trimpath -ldflags="-X main.appVersion=${APP_VERSION} -X main.appRevision=${APP_REVISION}" -o /out/qsobook . \
    && install -d -o 10001 -g 10001 -m 0700 /out/data

FROM scratch
LABEL org.opencontainers.image.title="QSObook" \
      org.opencontainers.image.description="A local amateur radio notebook" \
      org.opencontainers.image.source="https://github.com/erickfaria/QSObook" \
      org.opencontainers.image.licenses="AGPL-3.0"
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/qsobook /qsobook
COPY --from=build --chown=10001:10001 /out/data /data
USER 10001:10001
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/qsobook"]
CMD ["-container", "-addr", "0.0.0.0:8080", "-db", "/data/qsobook.db"]
