# The render server: the designer on / and POST /render, from one static binary.
#   docker run -p 8080:8080 -e DDREPORT_TOKEN=... ghcr.io/gvjavx/ddreport
# Fonts of your own: mount a folder and set DDREPORT_FONTS to it.
FROM --platform=$BUILDPLATFORM golang:1.26.10 AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY . .
# pure Go: built for the target system on the build machine, no emulation
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /ddreport ./cmd/ddreport

FROM scratch
COPY --from=build /ddreport /ddreport
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/ddreport"]
CMD ["-serve", "0.0.0.0:8080"]
