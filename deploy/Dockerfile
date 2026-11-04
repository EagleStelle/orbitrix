# syntax=docker/dockerfile:1
# The website and signaling server image, for linux/amd64 and linux/arm64. Build it from the repo root with
# `go tool mage image`, or without Go:
#   docker buildx build --platform linux/amd64,linux/arm64 -t eaglestelle/orbitrix-server -f deploy/Dockerfile --load .
# Both platforms build natively on the build machine, without emulation: the web build is wasm, and the server
# cross-compiles. Loading both into Docker takes its containerd image store (the default in Docker Desktop and in
# Docker Engine 29 and later).
FROM --platform=$BUILDPLATFORM golang:1.27.3 AS build
WORKDIR /src
COPY . .
# The same for every platform, so buildx runs it once.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go tool mage web
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /orbitrix-server ./cmd/orbitrix-server

# A static Go binary needs only CA roots (for Cloudflare's TURN API) beside it; distroless's tzdata and the rest
# would be dead weight.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /orbitrix-server /orbitrix-server
COPY --from=build /src/dist/web /web
# distroless's nonroot user.
USER 65532:65532
EXPOSE 1212
ENTRYPOINT ["/orbitrix-server", "-web", "/web", "-downloads", "/downloads"]

# Last, so a new version, commit or build time only relabels the image. `go tool mage image` passes all three; a plain
# build leaves them empty.
ARG VERSION REVISION CREATED
LABEL org.opencontainers.image.title="orbitrix-server" \
      org.opencontainers.image.description="Orbitrix website and signaling server: the landing page, the web game, room codes, join approval and TURN credentials. No game traffic passes through it." \
      org.opencontainers.image.authors="EagleStelle (https://github.com/EagleStelle)" \
      org.opencontainers.image.vendor="EagleStelle" \
      org.opencontainers.image.url="https://github.com/EagleStelle/orbitrix" \
      org.opencontainers.image.source="https://github.com/EagleStelle/orbitrix" \
      org.opencontainers.image.documentation="https://github.com/EagleStelle/orbitrix/blob/main/docs/deployment.md" \
      org.opencontainers.image.licenses="GPL-3.0-only" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION" \
      org.opencontainers.image.created="$CREATED"
