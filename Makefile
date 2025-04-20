# ---------------------------
#  Configurable Variables
# ---------------------------
ifdef CI_COMMIT_REF_NAME
  BRANCH_OR_TAG := $(CI_COMMIT_REF_NAME)
else
  BRANCH_OR_TAG := develop
endif

VERSION   := $(shell git rev-parse --short HEAD)
LDFLAGS   := -X 'github.com/NorkzYT/comic-downloader/cmd/comic-downloader.Version=$(VERSION)' \
             -X 'github.com/NorkzYT/comic-downloader/cmd/comic-downloader.Tag=$(BRANCH_OR_TAG)'
GOFLAGS   := -ldflags="$(LDFLAGS)"

BUILD_DIR    := build
BINARY_NAME  := comic-downloader
BINARY_WIN   := $(BINARY_NAME).exe
CMD_DIR      := ./cmd/comic-downloader
OUTPUT_DIR   := downloads

# use richgo if available for pretty test output
RICHGO := $(shell command -v richgo 2>/dev/null)

# ---------------------------
#  Default Target
# ---------------------------
.DEFAULT_GOAL := help

# ---------------------------
#  Phony Targets
# ---------------------------
.PHONY: help check install build build-all build/unix build/win test grabber \
        grabber/asurascans grabber/cypherscans grabber/inmanga \
        grabber/mangadex grabber/mangamonk grabber/reaperscans clean \
		up down

# ---------------------------
#  Help
# ---------------------------
help:
	@echo "\n\033[1;35mComic‑Downloader Makefile\033[0m"
	@echo "Version: $(VERSION)  Branch/Tag: $(BRANCH_OR_TAG)"
	@echo "--------------------------------------------------------------"
	@echo "\033[1;33mMain Commands:\033[0m"
	@echo "  make check         Verify that 'go' and 'git' are installed."
	@echo "  make install       Download Go modules (go mod download)."
	@echo "  make build         Clean, test, and build for current OS."
	@echo "  make build-all     Clean, test, and build for Unix + Windows."
	@echo "  make test          Run unit tests."
	@echo "  make grabber       Run quick smoke‑tests for each grabber."
	@echo "  make clean         Remove build artifacts and binaries."
	@echo "--------------------------------------------------------------\n"

# ---------------------------
#  Dependency Check
# ---------------------------
check:
	@command -v go  >/dev/null 2>&1 || { echo "Error: 'go' is not installed."; exit 1; }
	@command -v git >/dev/null 2>&1 || { echo "Error: 'git' is not installed."; exit 1; }
	@echo "✔ Dependencies: go, git"

# ---------------------------
#  Module Install
# ---------------------------
install: check
	@echo "→ Downloading modules..."
	go mod download

# ---------------------------
#  Build
# ---------------------------
build: clean test build/unix

build-all: clean test build/unix build/win

build/unix: check
	@echo "→ Building for Unix..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

build/win: check
	@echo "→ Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_WIN) $(CMD_DIR)

# ---------------------------
#  Tests
# ---------------------------
test: check
ifdef RICHGO
	@echo "→ Running tests with richgo..."
	richgo test -v ./...
else
	@echo "→ Running tests..."
	go test -v ./...
endif

# ---------------------------
#  Grabber Smoke‑Tests
# ---------------------------
grabber: grabber/asurascans grabber/cypherscans grabber/inmanga \
         grabber/mangadex grabber/mangamonk grabber/reaperscans

grabber/asurascans:
	@echo "→ Testing AsuraScans grabber..."
	go run $(CMD_DIR) https://asuracomic.net/series/player-who-returned-10000-years-later-44b620ed 1-2 \
		--format raw --output-dir $(OUTPUT_DIR)

grabber/cypherscans:
	@echo "→ Testing CypherScans grabber..."
	go run $(CMD_DIR) https://cypheroscans.xyz/manga/magic-emperor/ 1-2 \
		--format raw --output-dir $(OUTPUT_DIR)

grabber/inmanga:
	@echo "→ Testing Inmanga grabber..."
	go run $(CMD_DIR) https://inmanga.com/ver/manga/Kaiju-No-8/646317fc-f37c-4686-b568-df8efc60285d 1-2 \
		--format raw --output-dir $(OUTPUT_DIR)

grabber/mangadex:
	@echo "→ Testing MangaDex grabber..."
	go run $(CMD_DIR) https://mangadex.org/title/a1c7c817-4e59-43b7-9365-09675a149a6f/one-piece \
		--language en 1-2 --format raw --output-dir $(OUTPUT_DIR) --bundle

grabber/mangamonk:
	@echo "→ Testing MangaMonk grabber..."
	go run $(CMD_DIR) https://mangamonk.com/infinite-mage 1-2 \
		--format raw --output-dir $(OUTPUT_DIR)

grabber/reaperscans:
	@echo "→ Testing ReaperScans grabber..."
	go run $(CMD_DIR) https://reaperscans.com/series/the-100th-regression-of-the-max-level-player 1-2 \
		--format raw --output-dir $(OUTPUT_DIR)

# ---------------------------
#  Clean Up
# ---------------------------
clean:
	@echo "→ Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(BINARY_NAME)* *.cbz


# ---------------------------
#  Docker Compose Helpers
# ---------------------------

# Start Browserless + comic-downloader (Tenshi) stack
up:
	@echo "→ Starting Browserless and Tenshi containers..."
	docker compose -f docker-compose.yml up -d --force-recreate

# Tear down the stack
down:
	@echo "→ Stopping and removing containers..."
	docker compose -f docker-compose.yml down