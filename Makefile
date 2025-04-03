.PHONY: clean install build build/all build/unix build/win test grabber grabber/asurascans grabber/cypherscans grabber/inmanga grabber/mangadex grabber/mangamonk grabber/reaperscans

ifdef CI_COMMIT_REF_NAME
	BRANCH_OR_TAG := $(CI_COMMIT_REF_NAME)
else
	BRANCH_OR_TAG := develop
endif

VERSION := $(shell git rev-parse --short HEAD)
GOLDFLAGS += -X 'github.com/NorkzYT/comic-downloader/cmd/comic-downloader.Version=$(VERSION)'
GOLDFLAGS += -X 'github.com/NorkzYT/comic-downloader/cmd/comic-downloader.Tag=$(BRANCH_OR_TAG)'
GOFLAGS = -ldflags="$(GOLDFLAGS)"

RICHGO := $(shell command -v richgo 2> /dev/null)

# Directories and binary names
BUILD_DIR := build
BINARY_NAME := comic-downloader
BINARY_WIN := $(BINARY_NAME).exe
CMD_DIR := ./cmd/comic-downloader
OUTPUT_DIR := ./output

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(BINARY_NAME)* *.cbz

install:
	@echo "Downloading modules..."
	go mod download

build: clean test build/unix

build/all: clean test build/unix build/win

build/unix:
	@echo "Building for Unix..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_NAME) $(GOFLAGS) $(CMD_DIR)

build/win:
	@echo "Building for Windows..."
	mkdir -p $(BUILD_DIR)
	GOOS=windows go build -o $(BUILD_DIR)/$(BINARY_WIN) $(GOFLAGS) $(CMD_DIR)

test:
ifdef RICHGO
	@echo "Running tests with richgo..."
	richgo test -v ./...
else
	@echo "Running tests..."
	go test -v ./...
endif

# Grabber targets: run the binary with different URLs and options.
grabber: grabber/inmanga grabber/mangadex grabber/asurascans grabber/cypherscans grabber/mangamonk grabber/reaperscans

grabber/asurascans:
	@echo "Running asurascans grabber test..."
	go run $(CMD_DIR) https://asuracomic.net/series/player-who-returned-10000-years-later-44b620ed 1-2 --format raw --output-dir $(OUTPUT_DIR)

grabber/cypherscans:
	@echo "Running cypherscans grabber test..."
	go run $(CMD_DIR) https://cypheroscans.xyz/manga/magic-emperor/ 1-2 --format raw --output-dir $(OUTPUT_DIR)

grabber/inmanga:
	@echo "Running inmanga grabber test..."
	go run $(CMD_DIR) https://inmanga.com/ver/manga/Kaiju-No-8/646317fc-f37c-4686-b568-df8efc60285d 1-2 --format raw --output-dir $(OUTPUT_DIR)

grabber/mangadex:
	@echo "Running mangadex grabber test..."
	go run $(CMD_DIR) https://mangadex.org/title/a1c7c817-4e59-43b7-9365-09675a149a6f/one-piece --language en 1-2 --format raw --output-dir $(OUTPUT_DIR) --bundle

grabber/mangamonk:
	@echo "Running mangamonk grabber test..."
	go run $(CMD_DIR) https://mangamonk.com/infinite-mage 1-2 --format raw --output-dir $(OUTPUT_DIR)

grabber/reaperscans:
	@echo "Running reaperscans grabber test..."
	go run $(CMD_DIR) https://reaperscans.com/series/the-100th-regression-of-the-max-level-player 1-2 --format raw --output-dir $(OUTPUT_DIR)
