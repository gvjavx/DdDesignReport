# The render server: the designer on / and POST /render, from one static binary.
#   docker run -p 8080:8080 -e DDREPORT_TOKEN=... ghcr.io/gvjavx/ddreport
# Fonts of your own: mount a folder and set DDREPORT_FONTS to it (node fontpack/download.mjs gets 100 more).
# The designer's My templates are kept in /data: mount a volume there (-v ddreport:/data) to keep them.
FROM --platform=$BUILDPLATFORM golang:1.26.10 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
# pure Go: built for the target system on the build machine, no emulation
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /ddreport ./cmd/ddreport
RUN mkdir /data

FROM scratch
COPY --from=build /ddreport /ddreport
COPY --from=build --chown=65534:65534 /data /data
VOLUME /data
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/ddreport"]
CMD ["-serve", "0.0.0.0:8080", "-templates", "/data"]
