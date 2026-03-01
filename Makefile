.PHONY: build ui dev install uninstall clean package test dev-vm dev-vm-* dev-vm-down

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

# Clean
clean:
	rm -rf bin/
	rm -rf ui/greetdeez/build ui/greetdeez/.svelte-kit
	rm -rf dist/

# Dev
dev-vm: dev-vm-arch

dev-vm-down:
	vagrant destroy -f

dev-vm-%: dev-vm-down
	trap 'vagrant destroy -f' EXIT; \
	vagrant up $* && virt-viewer -c qemu:///system --wait GreetDeez_$*

# Package (via goreleaser)
package: build
	goreleaser release --snapshot --clean
