# syntax=docker/dockerfile:1
# The website and signaling server image. Build it from the repo root with `go tool mage image`, or without Go:
#   docker build -t orbitrix-server -f deploy/Dockerfile .
# Add --platform linux/arm64 for an ARM server: both builds run natively on the build machine (the web build is wasm,
# and the server cross-compiles).
FROM --platform=$BUILDPLATFORM golang:1.27.3 AS build
WORKDIR /src
COPY . .
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go tool mage web && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /orbitrix-server ./cmd/orbitrix-server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /orbitrix-server /orbitrix-server
COPY --from=build /src/dist/web /web
ENTRYPOINT ["/orbitrix-server", "-web", "/web", "-downloads", "/downloads"]
