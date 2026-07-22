#!/bin/sh
# Vagrant shim
set -eu

if command -v vagrant >/dev/null 2>&1; then
    exec vagrant "$@"
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "error: neither vagrant nor docker is installed" >&2
    exit 1
fi

IMAGE="${VAGRANT_LIBVIRT_IMAGE:-vagrantlibvirt/vagrant-libvirt:latest}"
VAGRANT_D="${VAGRANT_HOME:-$HOME/.vagrant.d}"
mkdir -p "$VAGRANT_D"

TTY_FLAG=""
if [ -t 0 ]; then
    TTY_FLAG="-t"
fi

# Root inside the container w/ libvirtd
exec docker run -i $TTY_FLAG --rm \
    --network host \
    --env LIBVIRT_DEFAULT_URI="${LIBVIRT_DEFAULT_URI:-qemu:///system}" \
    --env USER_UID=0 --env USER_GID=0 --env IGNORE_RUN_AS_ROOT=1 \
    --volume /var/run/libvirt/:/var/run/libvirt/ \
    --volume "$VAGRANT_D:/.vagrant.d" \
    --volume "$PWD:$PWD" \
    --workdir "$PWD" \
    "$IMAGE" \
    bash -c 'vagrant "$@"; rc=$?; chown -R '"$(id -u):$(id -g)"' /.vagrant.d "$PWD/.vagrant" 2>/dev/null; exit $rc' -- "$@"
