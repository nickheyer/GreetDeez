.PHONY: build ui mockgreetd dev dev-quick dev-greetd dev-greetd-build install uninstall clean package test deploy-vm

# override CGO flags w/ latest wv. webview_go hardcodes pkg-config webkit2gtk-4.0, but most distros ship 4.1
WEBKIT_PKG := $(shell pkg-config --exists webkit2gtk-4.1 2>/dev/null && echo webkit2gtk-4.1 || echo webkit2gtk-4.0)
CGO_CXXFLAGS_EXTRA := $(shell pkg-config --cflags $(WEBKIT_PKG) gtk+-3.0)
CGO_LDFLAGS_EXTRA := $(shell pkg-config --libs $(WEBKIT_PKG) gtk+-3.0)

# Build
build: ui
	@mkdir -p bin
	CGO_ENABLED=1 \
	CGO_CFLAGS="$(CGO_CXXFLAGS_EXTRA)" \
	CGO_CXXFLAGS="$(CGO_CXXFLAGS_EXTRA)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS_EXTRA)" \
	go build -o bin/greetdeez ./cmd/greetdeez

ui:
	cd ui/greetdeez && npm install && npm run build

mockgreetd:
	@mkdir -p bin
	go build -o bin/mockgreetd ./cmd/mockgreetd

# Dev tiers

# T1 - ui only
dev:
	cd ui/greetdeez && npm run dev

# T2 - mock greetd + go runtime
dev-quick: mockgreetd build
	@echo "Starting mock greetd..."
	@SOCK=$$(mktemp -u /tmp/greetd.XXXXXX.sock); \
	./bin/mockgreetd -sock "$$SOCK" -users "test:test,demo:demo" & \
	MOCK_PID=$$!; \
	sleep 0.5; \
	GREETD_SOCK="$$SOCK" \
	GREETDEEZ_SESSION_DIRS="wayland=testdata/sessions/wayland:x11=testdata/sessions/x11" \
	./bin/greetdeez -dev; \
	kill $$MOCK_PID 2>/dev/null || true

# T3 - Docker + greetd-stub + headless sway + wayvnc + greeter (VNC :5910)
dev-greetd: dev-greetd-build build
	docker compose -f docker-compose.dev.yml up

dev-greetd-build:
	docker compose -f docker-compose.dev.yml build

# Install
DESTDIR ?=
PREFIX ?= /usr/local

install: build
	install -Dm755 bin/greetdeez $(DESTDIR)$(PREFIX)/bin/greetdeez
	install -Dm644 greetd.toml $(DESTDIR)/etc/greetd/greetd.toml
	install -Dm644 greetdeez.conf $(DESTDIR)/etc/greetd/greetdeez.conf

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/greetdeez
	rm -f $(DESTDIR)/etc/greetd/greetd.toml
	rm -f $(DESTDIR)/etc/greetd/greetdeez.conf

# Test
test:
	go test ./...

# Misc
clean:
	rm -rf bin/
	rm -rf ui/greetdeez/build ui/greetdeez/.svelte-kit
	rm -rf dist/

# Deploy to dev VM (KVM/QEMU) via 9p shared filesystem.
# The VM must mount the shared dir (e.g. /mnt/greetdeez).
# Set VM_HOST to the VM's SSH address if you want auto-restart:
#   make deploy-vm VM_HOST=dev-vm
VM_HOST ?=
VM_MOUNT ?= /mnt/greetdeez

deploy-vm: build
	@mkdir -p dist
	cp bin/greetdeez dist/
	cp greetd.toml greetdeez.conf dist/
	@echo "Deployed to dist/ (shared via 9p at $(VM_MOUNT))"
ifdef VM_HOST
	ssh $(VM_HOST) 'sudo cp $(VM_MOUNT)/greetdeez /usr/bin/greetdeez && \
		sudo cp $(VM_MOUNT)/greetdeez.conf /etc/greetd/greetdeez.conf && \
		sudo systemctl restart greetd'
	@echo "Restarted greetd on $(VM_HOST)"
endif

# Package (via goreleaser)
package: build
	goreleaser release --snapshot --clean
