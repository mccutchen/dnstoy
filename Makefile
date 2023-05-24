# Built binaries will be placed here
OUT_DIR ?= bin

# Default arguments used by build and test targets
BUILD_ARGS    ?= -ldflags="-s -w"
TEST_ARGS     ?= -race
COVERAGE_PATH ?= coverage.txt
COVERAGE_ARGS ?= -covermode=atomic -coverprofile=$(COVERAGE_PATH)

# 3rd party tools
GOLINT      := go run golang.org/x/lint/golint@latest
REFLEX      := go run github.com/cespare/reflex@v0.3.1
STATICCHECK := go run honnef.co/go/tools/cmd/staticcheck@2023.1.3


# =============================================================================
# build
# =============================================================================

# Find commands to build
CMD_NAMES := $(shell ls cmd)                       # -> foo bar
CMD_PATHS := $(addprefix $(OUT_DIR)/,$(CMD_NAMES)) # -> $(OUT_DIR)/foo $(OUT_DIR)/bar

# Build every command we found
build: clean $(CMD_PATHS)
.PHONY: build

# Build a single command
$(OUT_DIR)/%:
	@ mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 go build $(BUILD_ARGS) -o $(OUT_DIR)/$(notdir $@) ./cmd/$(notdir $@)

clean:
	rm -rf $(OUT_DIR) $(COVERAGE_PATH)
.PHONY: clean


# =============================================================================
# test & lint
# =============================================================================
test:
	go test $(TEST_ARGS) ./...
.PHONY: test

testcover:
	go test $(TEST_ARGS) $(COVERAGE_ARGS) ./...
	go tool cover -html=$(COVERAGE_PATH)
.PHONY: testcover

lint:
	test -z "$$(gofmt -d -s -e .)" || (echo "Error: gofmt failed"; gofmt -d -s -e . ; exit 1)
	go vet ./...
	$(GOLINT) -set_exit_status ./...
	$(STATICCHECK) ./...
.PHONY: lint

run: build
	$(OUT_DIR)/testquery
