.PHONY: build gen buf-image proto-lint proto-format proto-breaking tools ui dev install uninstall clean package test dev-vm dev-vm-* dev-vm-down vm-base-*

# Code gen
BUF_IMAGE := greetdeez-buf
BUF_RUN := docker run --rm \
	--volume "$(shell pwd):/workspace" \
	--workdir /workspace \
	--user "$(shell id -u):$(shell id -g)" \
	--env HOME=/tmp \
	$(BUF_IMAGE)

buf-image:
	docker build -t $(BUF_IMAGE) -f docker/Dockerfile.buf .

gen: buf-image
	$(BUF_RUN) sh -c 'go install ./cmd/protoc-gen-greetdeez-go ./cmd/protoc-gen-greetdeez-es && buf generate'

proto-lint: buf-image
	$(BUF_RUN) buf lint

proto-format: buf-image
	$(BUF_RUN) buf format -w

proto-breaking: buf-image
	$(BUF_RUN) buf breaking --against '.git#branch=main'


# Dev tooling
tools:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/goreleaser/goreleaser/v2@latest
	@npm install -g @bufbuild/protoc-gen-es \
		|| echo "protoc-gen-es not installed (npm -g failed); only needed for running buf natively, gen uses Docker"

# Build
build: gen ui
	@mkdir -p bin
	CGO_ENABLED=1 go build -o bin/greetdeez ./cmd/greetdeez

ui:
	cd npm/proto && npm install && npm run build
	go generate ./ui/...

# Dev
dev: build
	cage -s -- ./bin/greetdeez -dev

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
	rm -rf npm/proto/dist
	rm -rf gen/
	rm -rf dist/

# Dev VMs
VAGRANT := ./scripts/vagrant.sh

dev-vm: dev-vm-arch

dev-vm-down:
	$(VAGRANT) destroy -f

# ensures base snapshot exists for vm
vm-base-%:
	@if ! $(VAGRANT) snapshot list $* 2>/dev/null | grep -q 'base'; then \
		$(VAGRANT) up $* --provision-with base && \
		$(VAGRANT) snapshot save $* base && \
		$(VAGRANT) halt $*; \
	fi

dev-vm-%: vm-base-%
	trap '$(VAGRANT) halt $*' EXIT; \
	$(VAGRANT) snapshot restore $* base && \
	$(VAGRANT) provision $* --provision-with package && \
	virt-viewer -c qemu:///system --wait --attach GreetDeez_$*

# Package (via goreleaser)
package: build
	goreleaser release --snapshot --clean
