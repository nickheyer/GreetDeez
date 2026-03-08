.PHONY: build gen ui dev install uninstall clean package test dev-vm dev-vm-* dev-vm-down

# Code gen
gen:
	go install ./cmd/protoc-gen-greetdeez-go
	go install ./cmd/protoc-gen-greetdeez-es
	buf generate

# Build
build: gen ui
	@mkdir -p bin
	CGO_ENABLED=1 go build -o bin/greetdeez ./cmd/greetdeez

ui:
	cd npm/proto && npm install && npm run build
	go generate ./ui/...

# Dev
dev:
	cd ui/cyber && npm run dev

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
	rm -rf ui/minimal/build
	rm -rf ui/cyber/build
	rm -rf ui/doom/build
	rm -rf npm/proto/dist
	rm -rf gen/
	rm -rf dist/

# Dev VMs
dev-vm: dev-vm-arch

dev-vm-down:
	vagrant destroy -f

dev-vm-%:
	@if ! vagrant snapshot list $* 2>/dev/null | grep -q 'base'; then \
		vagrant up $* --provision-with base && \
		vagrant snapshot save $* base && \
		vagrant halt $*; \
	fi
	trap 'vagrant halt $*' EXIT; \
	vagrant snapshot restore $* base && \
	vagrant provision $* --provision-with package && \
	virt-viewer -c qemu:///system --wait --attach GreetDeez_$*

# Package (via goreleaser)
package: build
	goreleaser release --snapshot --clean
