FROM golang:1.27-bookworm AS build

WORKDIR /src
COPY apps/control-plane ./apps/control-plane

WORKDIR /src/apps/control-plane
RUN GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/control-plane ./cmd/control-plane

FROM debian:13-slim

COPY --from=build /out/control-plane /usr/local/bin/guardian-control-plane
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/guardian-control-plane"]
