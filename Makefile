GO ?= go
GOCACHE ?= /private/tmp/agent-deck-go-build
GOMODCACHE ?= /private/tmp/agent-deck-go-mod
DIST_DIR ?= dist
PACKAGE := ./cmd/agentdeck
ARM64_MAX_BYTES ?= 26214400
BUILDINFO_PACKAGE := github.com/kitdine/agent-deck/internal/buildinfo
VERSION_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
VERSION_SUFFIX := $(shell if git describe --exact-match --tags HEAD >/dev/null 2>&1 && git diff --quiet && git diff --cached --quiet; then echo ""; else echo "-dev"; fi)
VERSION ?= $(VERSION_TAG)$(VERSION_SUFFIX)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')
BUILD_LDFLAGS := -X "$(BUILDINFO_PACKAGE).Version=$(VERSION)"
BUILD_LDFLAGS += -X "$(BUILDINFO_PACKAGE).Commit=$(COMMIT)"
BUILD_LDFLAGS += -X "$(BUILDINFO_PACKAGE).Branch=$(BRANCH)"
BUILD_LDFLAGS += -X "$(BUILDINFO_PACKAGE).BuildTime=$(BUILD_TIME)"
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
DATADIR ?= $(PREFIX)/share/agentdeck
FORCE ?= 0
COMPLETION_SHELL ?= auto
COMPLETION_RC ?=

.PHONY: build build-all release-tag release-archive check-arm64-size check-install check-privacy check-release-distribution install uninstall release-verify clean test test-race vet verify

build:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -mod=vendor -trimpath -ldflags='$(BUILD_LDFLAGS)' -o $(DIST_DIR)/agentdeck $(PACKAGE)

# Release binaries link with -s -w so the archived artifact is the stripped
# binary the size gate measures; no post-link strip diverges ship from gate.
build-all:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=arm64 $(GO) build -mod=vendor -trimpath -ldflags='-s -w $(BUILD_LDFLAGS)' -o $(DIST_DIR)/agentdeck_darwin_arm64 $(PACKAGE)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=amd64 $(GO) build -mod=vendor -trimpath -ldflags='-s -w $(BUILD_LDFLAGS)' -o $(DIST_DIR)/agentdeck_darwin_amd64 $(PACKAGE)

release-tag:
	@test -n "$(TAG)" || { echo "TAG is required" >&2; exit 2; }
	@test -n "$(RELEASE_NOTES)" || { echo "RELEASE_NOTES is required" >&2; exit 2; }
	bash scripts/create-release-tag.sh "$(TAG)" "$(RELEASE_NOTES)"

release-archive: build-all
	bash scripts/release-archive.sh "$(DIST_DIR)" "$(VERSION)"

# Measures the exact arm64 binary release-archive packages, so the gate covers
# the shipped artifact rather than a separately stripped build.
check-arm64-size: build-all
	test $$(wc -c < $(DIST_DIR)/agentdeck_darwin_arm64) -le $(ARM64_MAX_BYTES)

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

check-release-distribution:
	bash scripts/test-release-distribution.sh

release-verify: verify build-all check-arm64-size check-install check-privacy check-release-distribution

clean:
	rm -rf $(DIST_DIR)
