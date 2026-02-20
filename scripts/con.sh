#!/bin/bash
set -e

export PATH="/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/bin:/sbin"

# Must be run as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo $0 $*"
    exit 1
fi

BRIDGE="br0"
SUBNET="10.88.0"
CONTAINER_BASE="/opt/containers"
IMAGE="debian:stable-slim"
SCRIPT_PATH=$(realpath "$0")

ensure_bridge() {
    nft insert rule inet firewalld filter_FORWARD iifname "br0" oifname "eth0" accept
    nft insert rule inet firewalld filter_FORWARD iifname "eth0" oifname "br0" accept
    # firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i "$BRIDGE" -o eth0 -j ACCEPT
    # firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i eth0 -o "$BRIDGE" -j ACCEPT
    # firewall-cmd --reload

    if ! ip link show "$BRIDGE" &>/dev/null; then
        echo "Bridge $BRIDGE not found, creating..."
        ip link add "$BRIDGE" type bridge
        ip addr add "${SUBNET}.1/24" dev "$BRIDGE"
        ip link set "$BRIDGE" up
        sysctl -w net.ipv4.ip_forward=1 >/dev/null
        firewall-cmd --add-masquerade 2>/dev/null || true
        echo "Bridge $BRIDGE created"
    else
        echo "Bridge $BRIDGE already exists"
    fi
}

container_create() {
    ensure_bridge

    # 1. generate ID
    ID=$(head -c4 /dev/urandom | xxd -p)
    echo "Creating container: $ID"

    # 2. create directories
    CONTAINER_DIR="${CONTAINER_BASE}/${ID}"
    ROOTFS_DIR="${CONTAINER_DIR}/rootfs"
    mkdir -p "$ROOTFS_DIR"

    # 3. get rootfs from image
    echo "Pulling rootfs from ${IMAGE}..."
    crane export "$IMAGE" | tar -xC "$ROOTFS_DIR"

    # 4. create hostname/hosts/resolv.conf
    echo "container-${ID}" > "$CONTAINER_DIR/hostname"
    cat <<EOF > "$CONTAINER_DIR/hosts"
127.0.0.1   localhost container-${ID}
::1         localhost ip6-localhost ip6-loopback
EOF
    cp /etc/resolv.conf "$CONTAINER_DIR/resolv.conf"

    # 5. create named netns
    ip netns add "ctr-${ID}"

    # 6. create veth pair
    ip link add "veth-${ID}" type veth peer name "vctr-${ID}"

    # 7. attach host end to bridge
    ip link set "veth-${ID}" master "$BRIDGE"
    ip link set "veth-${ID}" up

    # 8. move container end into netns
    ip link set "vctr-${ID}" netns "ctr-${ID}"

    # 9. configure container-side networking
    ip netns exec "ctr-${ID}" ip link set lo up
    ip netns exec "ctr-${ID}" ip addr add "${SUBNET}.2/24" dev "vctr-${ID}"
    ip netns exec "ctr-${ID}" ip link set "vctr-${ID}" up
    ip netns exec "ctr-${ID}" ip route add default via "${SUBNET}.1"

    # 10. save container state for cleanup
    echo "$ID" > "$CONTAINER_DIR/id"
    echo "veth-${ID}" > "$CONTAINER_DIR/veth"

    echo "Container $ID ready"
    echo "  Network namespace: ctr-${ID}"
    echo "  IP: ${SUBNET}.2"
    echo "  Rootfs: $ROOTFS_DIR"

    # 11. launch container inside namespaces
    nsenter --net=/var/run/netns/ctr-${ID} \
    unshare --mount --pid --fork --uts --cgroup \
        env PATH="$PATH" \
        "$SCRIPT_PATH" --inner "$ID"
}

container_inner() {
    export PATH="/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/bin:/sbin"

    local ID=$1
    local CONTAINER_DIR="${CONTAINER_BASE}/${ID}"
    local ROOTFS_DIR="${CONTAINER_DIR}/rootfs"

    # isolate mount namespace
    mount --make-rslave /
    mount --rbind "$ROOTFS_DIR" "$ROOTFS_DIR"
    mount --make-private "$ROOTFS_DIR"

    # mount pseudo filesystems
    mkdir -p "$ROOTFS_DIR/proc"
    mount -t proc proc "$ROOTFS_DIR/proc"

    mkdir -p "$ROOTFS_DIR/dev"
    mount -t tmpfs -o nosuid,strictatime,mode=0755 tmpfs "$ROOTFS_DIR/dev"

    # minimal device nodes
    mknod -m 666 "$ROOTFS_DIR/dev/null"    c 1 3
    mknod -m 666 "$ROOTFS_DIR/dev/zero"    c 1 5
    mknod -m 666 "$ROOTFS_DIR/dev/random"  c 1 8
    mknod -m 666 "$ROOTFS_DIR/dev/full"    c 1 7
    mknod -m 666 "$ROOTFS_DIR/dev/urandom" c 1 9
    mknod -m 666 "$ROOTFS_DIR/dev/tty"     c 5 0
    ln -sf /proc/kcore     "$ROOTFS_DIR/dev/core"
    ln -sf /proc/self/fd   "$ROOTFS_DIR/dev/fd"
    ln -sf /proc/self/fd/0 "$ROOTFS_DIR/dev/stdin"
    ln -sf /proc/self/fd/1 "$ROOTFS_DIR/dev/stdout"
    ln -sf /proc/self/fd/2 "$ROOTFS_DIR/dev/stderr"

    chown root:root "$ROOTFS_DIR/dev/"{null,zero,full,random,urandom,tty}

    mkdir -p "$ROOTFS_DIR/dev/pts"
    mount -t devpts -o newinstance,ptmxmode=0666,mode=0620 devpts $ROOTFS_DIR/dev/pts
    ln -sf /dev/pts/ptmx "$ROOTFS_DIR/dev/ptmx"

    mkdir -p "$ROOTFS_DIR/dev/mqueue"
    mount -t mqueue -o nosuid,nodev,noexec mqueue $ROOTFS_DIR/dev/mqueue

    mkdir -p "$ROOTFS_DIR/dev/shm"
    mount -t tmpfs -o nosuid,nodev,noexec,mode=1777,size=67108864 tmpfs $ROOTFS_DIR/dev/shm

    mkdir -p "$ROOTFS_DIR/sys"
    mount -t sysfs -o ro,nosuid,nodev,noexec sysfs "$ROOTFS_DIR/sys"

    mkdir -p "$ROOTFS_DIR/sys/fs/cgroup"
    mount -t cgroup2 -o ro,nosuid,nodev,noexec cgroup2 "$ROOTFS_DIR/sys/fs/cgroup"

    # bind identity files
    for p in hostname hosts resolv.conf; do
        touch "$ROOTFS_DIR/etc/$p"
        mount --bind "$CONTAINER_DIR/$p" "$ROOTFS_DIR/etc/$p"
    done

    # pivot into rootfs
    cd "$ROOTFS_DIR"
    mkdir -p .oldroot
    pivot_root . .oldroot

    # clean up old root
    mount --make-rslave /
    umount -l .oldroot
    rmdir .oldroot

    # set hostname
    hostname "$(cat /etc/hostname)"

    # harden - make proc subdirs read-only
    for d in bus fs irq sys sysrq-trigger; do
        if [ -e "/proc/$d" ]; then
            mount --bind "/proc/$d" "/proc/$d"
            mount -o remount,bind,ro "/proc/$d"
        fi
    done

    # harden - mask sensitive paths
    for p in \
        /proc/asound \
        /proc/interrupts \
        /proc/kcore \
        /proc/keys \
        /proc/latency_stats \
        /proc/timer_list \
        /proc/timer_stats \
        /proc/sched_debug \
        /proc/acpi \
        /proc/scsi \
        /sys/firmware; do
        if [ -d "$p" ]; then
            mount -t tmpfs -o ro tmpfs "$p"
        elif [ -f "$p" ]; then
            mount --bind /dev/null "$p"
        fi
    done

    exec /bin/bash
}

container_destroy() {
    local ID=$1
    local CONTAINER_DIR="${CONTAINER_BASE}/${ID}"

    echo "Destroying container: $ID"

    # delete veth (deleting one end removes both)
    ip link del "veth-${ID}" 2>/dev/null || true

    # delete netns
    ip netns del "ctr-${ID}" 2>/dev/null || true

    # remove directories
    rm -rf "$CONTAINER_DIR"

    echo "Container $ID destroyed"
}

case "$1" in
    start)   container_create ;;
    stop)    container_destroy "$2" ;;
    --inner) container_inner "$2" ;;
    *)       echo "Usage: $0 start|stop <id>" ;;
esac
