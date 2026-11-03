# The website and signaling server image. It only packages what the build scripts' image target built into dist/
# (the context): nothing is compiled here.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --chmod=755 orbitrix-server-linux /orbitrix-server
COPY web /web
ENTRYPOINT ["/orbitrix-server", "-web", "/web", "-downloads", "/downloads"]
