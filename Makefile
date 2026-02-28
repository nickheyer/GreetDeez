.PHONY: build ui dev install uninstall clean package test dev-vm dev-vm-destroy

# NOTE: webview_go hardcodes `#cgo pkg-config: gtk+-3.0 webkit2gtk-4.0` in its
# Go source. CGO env vars are ADDITIVE to #cgo directives, not overrides.
# Adding webkit2gtk-4.1 flags here would link BOTH 4.0 (libsoup2) and 4.1
# (libsoup3), which crashes immediately. Let the library handle its own flags.

# Build
build: ui
	@mkdir -p bin
	CGO_ENABLED=1 go build -o bin/greetdeez ./cmd/greetdeez

ui:
	cd ui/greetdeez && npm install && npm run build

# Dev

dev:
	cd ui/greetdeez && npm run dev

# Install
DESTDIR ?=
PREFIX ?= /usr/local

install: build
	install -Dm755 bin/greetdeez $(DESTDIR)$(PREFIX)/bin/greetdeez
	install -Dm644 config/greetd.toml $(DESTDIR)/etc/greetd/greetd.toml
	install -Dm644 config/greetdeez.conf $(DESTDIR)/etc/greetd/greetdeez.conf
	install -Dm644 packaging/sysusers.d/greetdeez.conf $(DESTDIR)/usr/lib/sysusers.d/greetdeez.conf
	install -Dm644 packaging/tmpfiles.d/greetdeez.conf $(DESTDIR)/usr/lib/tmpfiles.d/greetdeez.conf
	install -Dm755 packaging/scripts/post-install.sh $(DESTDIR)/usr/share/greetdeez/post-install.sh
	@# Run post-install when installing directly (not into a DESTDIR staging root).
	@if [ -z "$(DESTDIR)" ]; then /usr/share/greetdeez/post-install.sh; fi

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/greetdeez
	rm -f $(DESTDIR)/etc/greetd/greetd.toml
	rm -f $(DESTDIR)/etc/greetd/greetdeez.conf
	rm -f $(DESTDIR)/usr/lib/sysusers.d/greetdeez.conf
	rm -f $(DESTDIR)/usr/lib/tmpfiles.d/greetdeez.conf
	rm -rf $(DESTDIR)/usr/share/greetdeez

# Test
test:
	go test ./...

# Misc
clean:
	rm -rf bin/
	rm -rf ui/greetdeez/build ui/greetdeez/.svelte-kit
	rm -rf dist/

# Dev VM (KVM/QEMU via libvirt + qemu-guest-agent, no SSH)
VM_NAME ?= ENDER_DEV_SYSTEM_01

dev-vm: build
	@mkdir -p dist
	cp bin/greetdeez dist/
	cp config/greetd.toml config/greetdeez.conf dist/
	VM_NAME=$(VM_NAME) ./scripts/dev-vm-deploy.sh

dev-vm-destroy:
	virsh -c qemu:///system destroy $(VM_NAME) 2>/dev/null || true
	virsh -c qemu:///system undefine $(VM_NAME) --remove-all-storage 2>/dev/null || true
	virsh -c qemu:///system vol-delete --pool default $(VM_NAME).qcow2 2>/dev/null || true
	virsh -c qemu:///system vol-delete --pool default $(VM_NAME)-cidata.iso 2>/dev/null || true
	@echo "VM $(VM_NAME) destroyed."

# Package (via goreleaser)
package: build
	goreleaser release --snapshot --clean
