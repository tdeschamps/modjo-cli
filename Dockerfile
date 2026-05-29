# Minimal runtime image; goreleaser injects the prebuilt static binary.
FROM gcr.io/distroless/static:nonroot
COPY modjo /usr/local/bin/modjo
ENTRYPOINT ["/usr/local/bin/modjo"]
