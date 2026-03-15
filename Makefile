GO ?= go
DIST_DIR ?= out
BINARY ?= wsl-backup
CMD_PATH ?= ./cmd/backup

.PHONY: all test test-unit test-integration build release container-build install clean

all: test build

test:
	$(GO) test ./...

test-unit:
	$(GO) test ./internal/...

test-integration:
	$(GO) test ./tests/integration/...

build:
	$(GO) build ./...

release: clean
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 $(GO) build -o $(DIST_DIR)/$(BINARY) $(CMD_PATH)

container-build:
	./scripts/build-in-container.sh

install:
	./scripts/install.sh

clean:
	rm -rf $(DIST_DIR)