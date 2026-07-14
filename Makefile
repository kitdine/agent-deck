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
INSTALL_MANIFEST := $(DATADIR)/install-manifest
INSTALL ?= install
SHA256 ?= shasum -a 256
FORCE ?= 0

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
	@set -eu; \
	mkdir -p "$(BINDIR)" "$(DATADIR)"; \
	bin_dir=$$(cd "$(BINDIR)" && pwd -P); \
	data_dir=$$(cd "$(DATADIR)" && pwd -P); \
	target="$$bin_dir/agentdeck"; \
	manifest="$$data_dir/install-manifest"; \
	if { [ -e "$$target" ] || [ -L "$$target" ]; } && { [ ! -f "$$target" ] || [ -L "$$target" ]; }; then \
		echo "refusing non-regular install target: $$target" >&2; exit 1; \
	fi; \
	if { [ -e "$$manifest" ] || [ -L "$$manifest" ]; } && { [ ! -f "$$manifest" ] || [ -L "$$manifest" ]; }; then \
		echo "refusing non-regular install manifest: $$manifest" >&2; exit 1; \
	fi; \
	if { [ -e "$$target" ] || [ -e "$$manifest" ]; } && [ "$(FORCE)" != "1" ]; then \
		echo "refusing to overwrite existing AgentDeck installation; rerun with FORCE=1" >&2; exit 1; \
	fi; \
	temporary=$$(mktemp "$$bin_dir/.agentdeck.install.XXXXXX"); \
	manifest_temporary=$$(mktemp "$$data_dir/.install-manifest.XXXXXX"); \
	trap '/bin/rm -f "$$temporary" "$$manifest_temporary"' EXIT HUP INT TERM; \
	$(INSTALL) -m 0755 "$(DIST_DIR)/agentdeck" "$$temporary"; \
	hash=$$($(SHA256) "$$temporary" | awk '{print $$1}'); \
	printf 'path=%s\nsha256=%s\n' "$$target" "$$hash" >"$$manifest_temporary"; \
	chmod 0644 "$$manifest_temporary"; \
	mv -f "$$temporary" "$$target"; \
	mv -f "$$manifest_temporary" "$$manifest"; \
	trap - EXIT HUP INT TERM; \
	echo "installed $$target"

uninstall:
	@set -eu; \
	manifest="$(INSTALL_MANIFEST)"; \
	if [ ! -f "$$manifest" ] || [ -L "$$manifest" ]; then \
		echo "valid AgentDeck install manifest not found: $$manifest" >&2; exit 1; \
	fi; \
	if [ "$$(wc -l <"$$manifest" | tr -d ' ')" != "2" ]; then \
		echo "invalid AgentDeck install manifest: $$manifest" >&2; exit 1; \
	fi; \
	path_line=$$(sed -n '1p' "$$manifest"); \
	hash_line=$$(sed -n '2p' "$$manifest"); \
	case "$$path_line" in path=*) target=$${path_line#path=} ;; *) echo "invalid install path record" >&2; exit 1 ;; esac; \
	case "$$hash_line" in sha256=*) recorded=$${hash_line#sha256=} ;; *) echo "invalid install hash record" >&2; exit 1 ;; esac; \
	case "$$recorded" in ''|*[!0-9a-f]*) echo "invalid install hash" >&2; exit 1 ;; esac; \
	if [ "$${#recorded}" -ne 64 ]; then echo "invalid install hash length" >&2; exit 1; fi; \
	if [ ! -d "$(BINDIR)" ]; then echo "install directory not found: $(BINDIR)" >&2; exit 1; fi; \
	expected=$$(cd "$(BINDIR)" && pwd -P)/agentdeck; \
	if [ "$$target" != "$$expected" ]; then echo "install manifest path mismatch" >&2; exit 1; fi; \
	if [ ! -f "$$target" ] || [ -L "$$target" ]; then echo "installed binary is not a regular file" >&2; exit 1; fi; \
	actual=$$($(SHA256) "$$target" | awk '{print $$1}'); \
	if [ "$$actual" != "$$recorded" ]; then echo "installed binary hash mismatch" >&2; exit 1; fi; \
	/bin/rm "$$target"; \
	/bin/rm "$$manifest"; \
	rmdir "$(DATADIR)" 2>/dev/null || :; \
	echo "uninstalled $$target; preserved AgentDeck user state"

check-install:
	bash scripts/test-install.sh

check-privacy:
	@bash scripts/check-privacy.sh

release-verify: verify build-all check-arm64-size check-install check-privacy

clean:
	rm -rf $(DIST_DIR)
