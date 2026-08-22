FROM debian:13-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install --no-install-recommends --yes \
        busybox \
        ca-certificates \
        curl \
        iproute2 \
        iputils-ping \
        nftables \
    && rm -rf /var/lib/apt/lists/*

COPY deploy/lab/assets/index.html /srv/index.html

CMD ["sleep", "infinity"]
