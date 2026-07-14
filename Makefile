GO ?= go
GOCACHE ?= /private/tmp/agent-deck-go-build
GOMODCACHE ?= /private/tmp/agent-deck-go-mod
DIST_DIR ?= dist
PACKAGE := ./cmd/agentdeck

.PHONY: build build-all clean test test-race vet verify verify-legacy

build:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -mod=vendor -trimpath -o $(DIST_DIR)/agentdeck $(PACKAGE)

build-all:
	mkdir -p $(DIST_DIR)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=arm64 $(GO) build -mod=vendor -trimpath -o $(DIST_DIR)/agentdeck_darwin_arm64 $(PACKAGE)
	env GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOOS=darwin GOARCH=amd64 $(GO) build -mod=vendor -trimpath -o $(DIST_DIR)/agentdeck_darwin_amd64 $(PACKAGE)

test:
	env GOCACHE=$(GOCACHE) $(GO) test -mod=vendor -count=1 ./...

test-race:
	env GOCACHE=$(GOCACHE) $(GO) test -mod=vendor -race -count=1 ./...

vet:
	env GOCACHE=$(GOCACHE) $(GO) vet -mod=vendor ./...

verify-legacy:
	bash tests/test-ai-provider-aliases.sh
	bash tests/test-ai-provider-mode.sh
	python3 -m unittest tests/test_ai_provider_usage.py
	python3 -m py_compile bin/ai-provider-mode bin/ai-provider-key bin/ai_provider_common.py bin/ai_provider_usage.py bin/ai-provider-usage bin/ai-provider-run bin/ai-provider-price-update

verify: test test-race vet verify-legacy

clean:
	rm -rf $(DIST_DIR)
