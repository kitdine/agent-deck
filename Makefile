GO ?= go
GOCACHE ?= /private/tmp/agent-deck-go-build
GOMODCACHE ?= /private/tmp/agent-deck-go-mod
GO_TEST_RUNNER := scripts/run-go-test.sh
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
APP_VERSION ?= $(patsubst v%,%,$(VERSION_TAG))
APP_BUILD_NUMBER ?=
MACOS_APP ?= apps/macos/build/DerivedData/Build/Products/Release/AgentDeck.app
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
DATADIR ?= $(PREFIX)/share/agentdeck
FORCE ?= 0
COMPLETION_SHELL ?= auto
COMPLETION_RC ?=

.PHONY: build build-all release-tag release-archive check-arm64-size check-go-test-runner check-install check-privacy check-release-distribution check-whitespace install uninstall release-verify clean test test-race vet verify prices-regen check-prices-reproducible test-macos-app build-macos-app build-macos-release package-macos-app check-widget-sandbox check-macos-distribution

.PHONY: release-artifact-verify

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

release-artifact-verify: release-archive
	bash scripts/verify-release-artifacts.sh "$(DIST_DIR)" "$(VERSION)" "$(COMMIT)"

# Measures the exact arm64 binary release-archive packages, so the gate covers
# the shipped artifact rather than a separately stripped build.
check-arm64-size: build-all
	test $$(wc -c < $(DIST_DIR)/agentdeck_darwin_arm64) -le $(ARM64_MAX_BYTES)

test:
	env GOCACHE=$(GOCACHE) GO_TEST_BIN=$(GO) $(GO_TEST_RUNNER) ./...

test-race:
	env GOCACHE=$(GOCACHE) GO_TEST_BIN=$(GO) $(GO_TEST_RUNNER) -race ./...

test-macos-app:
	bash scripts/test-macos-app.sh

build-macos-app:
	bash scripts/build-macos-app.sh

# The universal release candidate. It consumes build-all's binaries so the App,
# its embedded helper, and the CLI archives carry one version and one commit.
build-macos-release: build-all
	AGENTDECK_APP_CONFIGURATION=Release AGENTDECK_APP_VERSION="$(APP_VERSION)" \
		AGENTDECK_APP_BUILD_NUMBER="$(APP_BUILD_NUMBER)" \
		AGENTDECK_DIST_DIR="$(DIST_DIR)" bash scripts/build-macos-app.sh

# Signs, assembles, and (unless skipped) notarizes the built candidate. It
# publishes nothing; a real identity and notarization profile are supplied by
# the separately authorized release workflow, never by this target's defaults.
package-macos-app:
	bash scripts/package-macos-app.sh "$(MACOS_APP)" "$(VERSION)" "$(DIST_DIR)"

check-go-test-runner:
	bash scripts/test-run-go-test.sh

vet:
	env GOCACHE=$(GOCACHE) $(GO) vet -mod=vendor ./...

# Regenerate the bundled price catalog from a pinned LiteLLM commit, merging
# the curated gap-fill over it. Network-using and release-time only; normal
# builds and tests never run it. Pass LITELLM_COMMIT=<sha> to pin explicitly;
# omitting it resolves current LiteLLM main and reports the sha it pinned.
prices-regen:
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run -mod=vendor ./tools/genprices -commit "$(LITELLM_COMMIT)"

# Verify the committed catalog is exactly what regeneration from its recorded
# pinned commit produces, so a hand-edit cannot slip in. Requires network, so
# it stays out of `verify`/`release-verify` and is run deliberately.
check-prices-reproducible:
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run -mod=vendor ./tools/genprices -check

verify: check-whitespace check-go-test-runner test test-race vet

install: build
	@PREFIX="$(PREFIX)" BINDIR="$(BINDIR)" DATADIR="$(DATADIR)" FORCE="$(FORCE)" COMPLETION_SHELL="$(COMPLETION_SHELL)" COMPLETION_RC="$(COMPLETION_RC)" bash scripts/manage-install.sh install "$(DIST_DIR)/agentdeck"

uninstall:
	@PREFIX="$(PREFIX)" BINDIR="$(BINDIR)" DATADIR="$(DATADIR)" bash scripts/manage-install.sh uninstall

check-install:
	bash scripts/test-install.sh
	bash scripts/test-completion-install.sh
	bash scripts/test-shell-integration-acceptance.sh

check-privacy:
	@bash scripts/check-privacy.sh

# The static half of the widget's privacy proof. It is a gate rather than a
# one-off, so it runs from the aggregate release check and not only from the
# task that wrote it. The runtime half is a manual macOS acceptance step.
check-widget-sandbox:
	@bash scripts/check-widget-sandbox.sh

# Scans tracked and untracked content, not a diff, so a violation already
# committed stays visible instead of only surfacing in diffs that touch it.
check-whitespace:
	@bash scripts/check-whitespace.sh

# The topic-document audit is NOT a make target. It is a documentation-workflow
# tool that reads only docs/topics/**, participates in no build, `verify`, or
# release path, and a make alias for it only implied otherwise. Run it directly:
# `bash scripts/check-topic-docs.sh`; see docs/README.md.

check-release-distribution:
	bash scripts/test-release-distribution.sh
	bash scripts/test-release-preflight.sh

# Desktop distribution, in isolation: cask rendering, Homebrew's acceptance of
# the rendered cask, signing order, the notarization and stapling invocations,
# artifact assembly, Formula-to-Cask migration, and mutual exclusion. Reaches no
# Apple service and no published tap, and installs nothing. It does require
# Homebrew: the cask load check creates and removes a throwaway tap inside the
# local Homebrew prefix, and fails the run when `brew` is absent rather than
# skipping the only check that reads Homebrew's own verdict.
check-macos-distribution:
	bash scripts/test-macos-distribution.sh
	bash scripts/test-cask-migration.sh

release-verify: verify build-all check-arm64-size check-install check-privacy check-widget-sandbox check-release-distribution check-macos-distribution

clean:
	rm -rf $(DIST_DIR)
