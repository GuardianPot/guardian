#!/usr/bin/env bash
# Runs inside the throwaway privileged container `task privileged:containerd`
# starts. It installs a pinned containerd 2.x and runc, verifies both against
# checksums recorded here, starts containerd, and runs the P2-W3 lab against it.
#
# Privileged because containerd needs to mount, create cgroups, and create
# namespaces. The code under test gets none of that: it reaches containerd only
# through the socket, exactly as the helper does on a host.
set -euo pipefail

containerd_version=2.3.5
containerd_sha256=2f0a095a71e3262d0d91ff0e50e2e4ae73c3866c4d1ff6a15f341097fba3dd44
runc_version=1.5.1
runc_sha256=177df879d50c913eb205e898d5c1c05a18f574053c0ce5524c471208eaf06f6f

if [ "$(uname -m)" != "x86_64" ]; then
  echo "the containerd lab pins amd64 binaries and cannot run on $(uname -m)" >&2
  exit 1
fi

workdir="$(mktemp -d)"
curl -fsSLo "$workdir/containerd.tgz" \
  "https://github.com/containerd/containerd/releases/download/v${containerd_version}/containerd-${containerd_version}-linux-amd64.tar.gz"
echo "${containerd_sha256}  $workdir/containerd.tgz" | sha256sum -c - >/dev/null
curl -fsSLo "$workdir/runc" \
  "https://github.com/opencontainers/runc/releases/download/v${runc_version}/runc.amd64"
echo "${runc_sha256}  $workdir/runc" | sha256sum -c - >/dev/null
tar -xzf "$workdir/containerd.tgz" -C /usr/local
install -m 0755 "$workdir/runc" /usr/local/bin/runc

# cgroup v2 refuses to delegate controllers from a cgroup that holds processes,
# so move everything here into a leaf first and then enable the controllers the
# decoy's limits need.
mkdir -p /sys/fs/cgroup/init
xargs -rn1 </sys/fs/cgroup/cgroup.procs >/sys/fs/cgroup/init/cgroup.procs 2>/dev/null || :
sed -e 's/ / +/g' -e 's/^/+/' </sys/fs/cgroup/cgroup.controllers >/sys/fs/cgroup/cgroup.subtree_control

containerd >"$workdir/containerd.log" 2>&1 &
for _ in $(seq 100); do
  [ -S /run/containerd/containerd.sock ] && break
  sleep 0.1
done
if [ ! -S /run/containerd/containerd.sock ]; then
  tail -50 "$workdir/containerd.log" >&2
  exit 1
fi

cd /src
if ! go test -mod=readonly -count=1 -v \
  -run 'TestContainerdRuntimeAgainstARealContainerd|AgainstALiveKernel' ./internal/privileged; then
  echo '--- containerd log ---' >&2
  tail -80 "$workdir/containerd.log" >&2
  exit 1
fi
