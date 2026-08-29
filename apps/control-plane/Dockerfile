FROM node:24-bookworm@sha256:be23f54a88d34e8824c741b19b91064094f92c1c97b194144bfc8b50d67258e2 AS web-build

WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web-console/package.json ./apps/web-console/package.json
RUN npm ci
COPY apps/web-console ./apps/web-console
RUN npm run build --workspace @guardianpot/web-console \
    && npm run bundle:check --workspace @guardianpot/web-console

FROM golang:1.27-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466 AS build

WORKDIR /src
COPY apps/control-plane ./apps/control-plane

WORKDIR /src/apps/control-plane
RUN GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/control-plane ./cmd/control-plane

FROM gcr.io/distroless/static-debian13:nonroot@sha256:1c2c046bc09ed40fad370b599a0b1ae7987f55b01e247cf27a7c27cd97e5bbc7

COPY --from=build --chown=65532:65532 --chmod=0555 /out/control-plane /usr/local/bin/guardian-control-plane
COPY --from=web-build --chown=65532:65532 /src/apps/web-console/dist /usr/share/guardian/web-console
ENV GUARDIAN_WEB_CONSOLE_DIR=/usr/share/guardian/web-console
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/guardian-control-plane"]
CMD ["serve"]
