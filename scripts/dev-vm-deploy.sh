#!/usr/bin/env bash
set -euo pipefail

# Deploy GreetDeez to a libvirt VM via qemu-guest-agent.
# Creates the VM automatically from an Arch cloud image if it doesn't exist.
# No SSH. No sudo. No manual steps.
#
# Usage: VM_NAME=ENDER_DEV_SYSTEM_01 ./scripts/dev-vm-deploy.sh

# --- configuration ---

VM_NAME="${VM_NAME:?VM_NAME must be set (e.g. ENDER_DEV_SYSTEM_01)}"
LIBVIRT_URI="${LIBVIRT_URI:-qemu:///system}"
SPICE_PORT="${SPICE_PORT:-5900}"
VM_MEMORY="${VM_MEMORY:-2048}"
VM_CPUS="${VM_CPUS:-2}"
VM_DISK_SIZE="${VM_DISK_SIZE:-10G}"
VM_BRIDGE="${VM_BRIDGE:-br0}"
AGENT_TIMEOUT="${AGENT_TIMEOUT:-180}"
POOL="${LIBVIRT_POOL:-default}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="$(realpath "$PROJECT_DIR/dist")"

VIRSH="virsh -c $LIBVIRT_URI"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/greetdeez"
ARCH_IMAGE_URL="https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2"

# --- helpers ---

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo ":: $*"; }

# Run a command inside the guest via qemu-guest-agent.
# Waits for completion, prints stdout/stderr, returns the guest exit code.
guest_exec() {
	local cmd="$1"
	shift
	local args=""
	for arg in "$@"; do
		args+="$(printf ', "%s"' "$arg")"
	done

	local exec_json
	exec_json=$($VIRSH qemu-agent-command "$VM_NAME" \
		"{\"execute\":\"guest-exec\",\"arguments\":{\"path\":\"$cmd\",\"arg\":[${args#, }],\"capture-output\":true}}" 2>&1) \
		|| die "guest-exec failed: $exec_json"

	local pid
	pid=$(echo "$exec_json" | jq -r '.return.pid') \
		|| die "failed to parse guest-exec response: $exec_json"

	# Poll for completion
	local status_json exited
	while true; do
		status_json=$($VIRSH qemu-agent-command "$VM_NAME" \
			"{\"execute\":\"guest-exec-status\",\"arguments\":{\"pid\":$pid}}" 2>&1) \
			|| die "guest-exec-status failed: $status_json"
		exited=$(echo "$status_json" | jq -r '.return.exited')
		[[ "$exited" == "true" ]] && break
		sleep 0.3
	done

	local exit_code out_b64 err_b64
	exit_code=$(echo "$status_json" | jq -r '.return.exitcode')
	out_b64=$(echo "$status_json" | jq -r '.return["out-data"] // empty')
	err_b64=$(echo "$status_json" | jq -r '.return["err-data"] // empty')

	[[ -n "$out_b64" ]] && echo "$out_b64" | base64 -d
	[[ -n "$err_b64" ]] && echo "$err_b64" | base64 -d >&2

	return "$exit_code"
}

# Find an ISO creation tool
find_mkiso() {
	for cmd in xorrisofs genisoimage mkisofs; do
		if command -v "$cmd" >/dev/null; then
			echo "$cmd"
			return
		fi
	done
	return 1
}

# --- VM creation ---

create_vm() {
	info "VM '$VM_NAME' not found — creating from Arch cloud image..."

	for cmd in qemu-img curl; do
		command -v "$cmd" >/dev/null || die "$cmd not found — install it"
	done
	local mkiso
	mkiso=$(find_mkiso) || die "No ISO tool found — install cdrtools or libisoburn"

	# Ensure storage pool is active
	if ! $VIRSH pool-info "$POOL" >/dev/null 2>&1; then
		info "Creating '$POOL' storage pool..."
		$VIRSH pool-define-as "$POOL" dir - - - - /var/lib/libvirt/images
		$VIRSH pool-build "$POOL"
		$VIRSH pool-start "$POOL"
		$VIRSH pool-autostart "$POOL"
	fi
	$VIRSH pool-start "$POOL" 2>/dev/null || true

	# Download cloud image (cached)
	mkdir -p "$CACHE_DIR"
	local cached_image="$CACHE_DIR/Arch-Linux-x86_64-cloudimg.qcow2"
	if [[ ! -f "$cached_image" ]]; then
		info "Downloading Arch Linux cloud image..."
		curl -L --progress-bar -o "${cached_image}.tmp" "$ARCH_IMAGE_URL"
		mv "${cached_image}.tmp" "$cached_image"
	else
		info "Using cached Arch cloud image."
	fi

	# Prepare resized disk image in a temp file
	info "Creating VM disk (${VM_DISK_SIZE})..."
	local tmp_disk
	tmp_disk=$(mktemp --suffix=.qcow2)
	cp "$cached_image" "$tmp_disk"
	qemu-img resize "$tmp_disk" "$VM_DISK_SIZE"

	# Upload disk to libvirt storage pool (no sudo needed)
	$VIRSH pool-refresh "$POOL" 2>/dev/null || true
	$VIRSH vol-delete --pool "$POOL" "${VM_NAME}.qcow2" 2>/dev/null || true
	local vol_bytes
	vol_bytes=$(stat -c %s "$tmp_disk")
	$VIRSH vol-create-as --pool "$POOL" --name "${VM_NAME}.qcow2" \
		--capacity "${vol_bytes}" --format raw >/dev/null
	$VIRSH vol-upload --pool "$POOL" "${VM_NAME}.qcow2" "$tmp_disk"
	rm -f "$tmp_disk"

	local vm_disk
	vm_disk=$($VIRSH vol-path --pool "$POOL" "${VM_NAME}.qcow2")

	# Create cloud-init ISO
	info "Generating cloud-init config..."
	local ci_dir
	ci_dir=$(mktemp -d)

	cat > "$ci_dir/user-data" <<'CLOUD_INIT'
#cloud-config
ssh_pwauth: false

package_update: true

packages:
  - qemu-guest-agent
  - greetd
  - cage
  - webkit2gtk
  - sway

write_files:
  - path: /etc/greetd/config.toml
    owner: root:root
    permissions: '0644'
    content: |
      [terminal]
      vt = 7

      [default_session]
      command = "cage -s -- /usr/bin/greetdeez"
      user = "greetdeez"

runcmd:
  - echo 'root:greetdeez' | chpasswd
  - useradd -m -s /bin/bash test && echo 'test:test' | chpasswd
  - mkdir -p /mnt/greetdeez
  - grep -q greetdeez /etc/fstab || echo 'greetdeez /mnt/greetdeez 9p trans=virtio,version=9p2000.L,_netdev 0 0' >> /etc/fstab
  - id greetdeez &>/dev/null || useradd -r -s /usr/bin/nologin greetdeez
  - usermod -aG video greetdeez
  - systemctl enable --now qemu-guest-agent
  - systemctl enable greetd
CLOUD_INIT

	cat > "$ci_dir/meta-data" <<META
instance-id: ${VM_NAME}
local-hostname: ${VM_NAME}
META

	local cidata_iso="$ci_dir/cidata.iso"
	"$mkiso" -output "$cidata_iso" -volid cidata -joliet -rational-rock \
		"$ci_dir/user-data" "$ci_dir/meta-data" >/dev/null 2>&1

	# Upload cidata ISO to libvirt pool (stale volumes already cleaned by pool-refresh above)
	$VIRSH vol-delete --pool "$POOL" "${VM_NAME}-cidata.iso" 2>/dev/null || true
	local iso_bytes
	iso_bytes=$(stat -c %s "$cidata_iso")
	$VIRSH vol-create-as --pool "$POOL" --name "${VM_NAME}-cidata.iso" \
		--capacity "${iso_bytes}" --format raw >/dev/null
	$VIRSH vol-upload --pool "$POOL" "${VM_NAME}-cidata.iso" "$cidata_iso"
	rm -rf "$ci_dir"

	local cidata_path
	cidata_path=$($VIRSH vol-path --pool "$POOL" "${VM_NAME}-cidata.iso")

	# Define VM with XML (no virt-install needed)
	info "Defining VM..."
	local vm_xml
	vm_xml=$(mktemp --suffix=.xml)
	cat > "$vm_xml" <<VMXML
<domain type='kvm'>
  <name>${VM_NAME}</name>
  <memory unit='MiB'>${VM_MEMORY}</memory>
  <vcpu>${VM_CPUS}</vcpu>
  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='${vm_disk}'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='${cidata_path}'/>
      <target dev='sda' bus='sata'/>
      <readonly/>
    </disk>
    <filesystem type='mount' accessmode='passthrough'>
      <source dir='${DIST_DIR}'/>
      <target dir='greetdeez'/>
    </filesystem>
    <graphics type='spice' port='${SPICE_PORT}' autoport='no' listen='127.0.0.1'>
      <listen type='address' address='127.0.0.1'/>
    </graphics>
    <channel type='unix'>
      <target type='virtio' name='org.qemu.guest_agent.0'/>
    </channel>
    <interface type='bridge'>
      <source bridge='${VM_BRIDGE}'/>
      <model type='virtio'/>
    </interface>
    <video>
      <model type='qxl' ram='65536' vram='65536' vgamem='16384'/>
    </video>
    <serial type='pty'/>
    <console type='pty'/>
  </devices>
</domain>
VMXML

	$VIRSH define "$vm_xml" >/dev/null
	rm -f "$vm_xml"

	$VIRSH start "$VM_NAME" >/dev/null
	info "VM created and started. Cloud-init provisioning..."
}

# --- main ---

# Preflight
command -v virsh >/dev/null || die "virsh not found — install libvirt"
command -v jq >/dev/null    || die "jq not found — install jq"

# Ensure VM exists — create from cloud image if not
if ! $VIRSH dominfo "$VM_NAME" >/dev/null 2>&1; then
	create_vm
fi

# Ensure VM is running
state=$($VIRSH domstate "$VM_NAME" 2>/dev/null | tr -d '[:space:]')
if [[ "$state" != "running" ]]; then
	info "Starting VM '$VM_NAME'..."
	$VIRSH start "$VM_NAME" >/dev/null
fi

# Wait for guest agent
info "Waiting for guest agent (timeout: ${AGENT_TIMEOUT}s)..."
deadline=$((SECONDS + AGENT_TIMEOUT))
agent_ready=false
while ((SECONDS < deadline)); do
	if $VIRSH qemu-agent-command "$VM_NAME" '{"execute":"guest-ping"}' >/dev/null 2>&1; then
		agent_ready=true
		break
	fi
	sleep 1
done

if [[ "$agent_ready" != "true" ]]; then
	die "Guest agent not responding after ${AGENT_TIMEOUT}s.
  If this is an old VM without qemu-guest-agent, recreate it:
    make dev-vm-destroy && make dev-vm"
fi
info "Guest agent connected."

# Wait for cloud-init to finish (returns immediately if already done)
info "Waiting for cloud-init..."
guest_exec /usr/bin/cloud-init "status" "--wait" || true

# --- deploy ---

info "Mounting 9p shares..."
guest_exec /usr/bin/mount "-a" || true

info "Copying binary..."
guest_exec /usr/bin/cp "/mnt/greetdeez/greetdeez" "/usr/bin/greetdeez"

info "Copying greetd config..."
guest_exec /usr/bin/cp "/mnt/greetdeez/greetd.toml" "/etc/greetd/config.toml"

info "Copying greeter config..."
guest_exec /usr/bin/cp "/mnt/greetdeez/greetdeez.conf" "/etc/greetd/greetdeez.conf"

info "Restarting greetd..."
guest_exec /usr/bin/systemctl "restart" "greetd" || true

# Health check — show greetd status so we can see if it crashed
sleep 1
info "greetd status:"
guest_exec /usr/bin/systemctl "status" "greetd" "--no-pager" || true

info "Deploy complete."

# --- open SPICE viewer ---

if command -v remote-viewer >/dev/null; then
	if ! pgrep -f "remote-viewer.*spice://127.0.0.1:${SPICE_PORT}" >/dev/null 2>&1; then
		info "Opening SPICE viewer on port ${SPICE_PORT}..."
		nohup remote-viewer "spice://127.0.0.1:${SPICE_PORT}" >/dev/null 2>&1 &
	else
		info "SPICE viewer already running."
	fi
else
	info "remote-viewer not found — connect manually: spice://127.0.0.1:${SPICE_PORT}"
fi
