# syntax=docker/dockerfile:1
# Cross-compiles on the build host (no QEMU) for the requested platform.
FROM --platform=$BUILDPLATFORM golang:1.27 AS build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X github.com/tmaegis/tmaegis/cmd.Version=${VERSION}" -o /out/tmaegis . \
 && GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tmaegis-server ./cmd/tmaegis-server

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="TMAegis" \
      org.opencontainers.image.authors="Tommy Huynh, Misha Milanovic" \
      org.opencontainers.image.licenses="MPL-2.0" \
      org.opencontainers.image.source="https://github.com/tmaegis/tmaegis"
COPY --from=build /out/ /usr/local/bin/
COPY rules /etc/tmaegis/rules
COPY plugins /etc/tmaegis/plugins
COPY samples /etc/tmaegis/samples
USER nonroot
ENTRYPOINT ["/usr/local/bin/tmaegis"]
CMD ["--help"]
