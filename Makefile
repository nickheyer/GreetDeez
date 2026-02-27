.PHONY: build ui mockgreetd dev dev-quick dev-greetd dev-greetd-build install uninstall clean package

# Build
build: ui
	@mkdir -p bin
	CGO_ENABLED=1 go build -o bin/greetdeez ./cmd/greetdeez

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

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/greetdeez
	rm -f $(DESTDIR)/etc/greetd/greetd.toml

# Misc
clean:
	rm -rf bin/
	rm -rf ui/greetdeez/build ui/greetdeez/.svelte-kit
	rm -rf dist/

# Package
NFPM := docker run --rm -v $(CURDIR):/tmp -w /tmp goreleaser/nfpm:latest package

package: build
	@mkdir -p dist
	$(NFPM) -p deb -t dist/
	$(NFPM) -p rpm -t dist/
	$(NFPM) -p archlinux -t dist/
	$(NFPM) -p apk -t dist/
