GO ?= go
GOCACHE ?= /private/tmp/agent-deck-go-build
GOMODCACHE ?= /private/tmp/agent-deck-go-mod
DIST_DIR ?= dist
PACKAGE := ./cmd/agentdeck
ARM64_MAX_BYTES ?= 26214400
BUILDINFO_PACKAGE := github.com/kitdine/agent-deck/internal/buildinfo
VERSION ?= dev
COMMIT ?= unknown
BUILD_TIME ?= unknown
BUILD_LDFLAGS := -X $(BUILDINFO_PACKAGE).Version=$(VERSION) -X $(BUILDINFO_PACKAGE).Commit=$(COMMIT) -X $(BUILDINFO_PACKAGE).BuildTime=$(BUILD_TIME)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
DATADIR ?= $(PREFIX)/share/agentdeck
FORCE ?= 0
COMPLETION_SHELL ?= auto
COMPLETION_RC ?=

.PHONY: build build-all build-arm64-stripped check-arm64-size check-install check-privacy install uninstall release-verify clean test test-race vet verify

build:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -mod=vendor -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(DIST_DIR)/agentdeck $(PACKAGE)

build-all:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=arm64 $(GO) build -mod=vendor -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(DIST_DIR)/agentdeck_darwin_arm64 $(PACKAGE)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=amd64 $(GO) build -mod=vendor -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(DIST_DIR)/agentdeck_darwin_amd64 $(PACKAGE)

build-arm64-stripped:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=arm64 $(GO) build -mod=vendor -trimpath -ldflags="-s -w $(BUILD_LDFLAGS)" -o $(DIST_DIR)/agentdeck_darwin_arm64_stripped $(PACKAGE)

check-arm64-size: build-arm64-stripped
	test $$(wc -c < $(DIST_DIR)/agentdeck_darwin_arm64_stripped) -le $(ARM64_MAX_BYTES)

test:
	env GOCACHE=$(GOCACHE) $(GO) test -mod=vendor -count=1 ./...

test-race:
	env GOCACHE=$(GOCACHE) $(GO) test -mod=vendor -race -count=1 ./...

vet:
	env GOCACHE=$(GOCACHE) $(GO) vet -mod=vendor ./...

verify: test test-race vet

install: build
	@PREFIX="$(PREFIX)" BINDIR="$(BINDIR)" DATADIR="$(DATADIR)" FORCE="$(FORCE)" COMPLETION_SHELL="$(COMPLETION_SHELL)" COMPLETION_RC="$(COMPLETION_RC)" bash scripts/manage-install.sh install "$(DIST_DIR)/agentdeck"

uninstall:
	@PREFIX="$(PREFIX)" BINDIR="$(BINDIR)" DATADIR="$(DATADIR)" bash scripts/manage-install.sh uninstall

check-install:
	bash scripts/test-install.sh
	bash scripts/test-completion-install.sh

check-privacy:
	@bash scripts/check-privacy.sh

release-verify: verify build-all check-arm64-size check-install check-privacy

clean:
	rm -rf $(DIST_DIR)
