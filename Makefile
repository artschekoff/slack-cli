# main.version must be a package-level var in the main package for -X to work.
.PHONY: build build-all pack install run test lint fmt vet vulncheck validate clean rulesync-install release

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BIN := bin/slack-cli
BIN_DIR := bin
DIST_DIR := $(BIN_DIR)/dist
CMD := ./cmd/slack-cli
INSTALL_DIR := /usr/local/bin
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BIN) $(CMD)

build-all:
	GOOS=darwin  GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/slack-cli-darwin-amd64 $(CMD)
	GOOS=darwin  GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/slack-cli-darwin-arm64 $(CMD)
	GOOS=linux   GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/slack-cli-linux-amd64 $(CMD)
	GOOS=linux   GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/slack-cli-linux-arm64 $(CMD)
	GOOS=windows GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(BIN_DIR)/slack-cli-windows-amd64.exe $(CMD)

pack: build-all
	mkdir -p $(DIST_DIR)
	tar -czf $(DIST_DIR)/slack-cli-darwin-amd64.tar.gz -C $(BIN_DIR) slack-cli-darwin-amd64
	tar -czf $(DIST_DIR)/slack-cli-darwin-arm64.tar.gz -C $(BIN_DIR) slack-cli-darwin-arm64
	tar -czf $(DIST_DIR)/slack-cli-linux-amd64.tar.gz  -C $(BIN_DIR) slack-cli-linux-amd64
	tar -czf $(DIST_DIR)/slack-cli-linux-arm64.tar.gz  -C $(BIN_DIR) slack-cli-linux-arm64
	zip -j   $(DIST_DIR)/slack-cli-windows-amd64.zip       $(BIN_DIR)/slack-cli-windows-amd64.exe

install: build
	sudo install -m 755 $(BIN) $(INSTALL_DIR)/slack-cli

run:
	go run $(CMD)

test:
	go test ./... -race -cover -count=1

lint:
	golangci-lint run

fmt:
	gofumpt -l -w .

vet:
	go vet ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

validate: fmt vet lint test vulncheck

clean:
	rm -rf bin/

release:
	@set -e; \
	current=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo "Current version: $$current"; \
	printf "Bump type? [major/minor/patch] (default: patch): "; \
	read bump; \
	bump=$${bump:-patch}; \
	ver=$${current#v}; \
	major=$$(echo $$ver | cut -d. -f1); \
	minor=$$(echo $$ver | cut -d. -f2); \
	patch=$$(echo $$ver | cut -d. -f3); \
	case $$bump in \
		major) major=$$((major+1)); minor=0; patch=0 ;; \
		minor) minor=$$((minor+1)); patch=0 ;; \
		*)     patch=$$((patch+1)) ;; \
	esac; \
	new="v$${major}.$${minor}.$${patch}"; \
	echo "Tagging and releasing $$new..."; \
	git tag $$new; \
	git push origin $$new; \
	$(MAKE) pack VERSION=$$new; \
	gh release create $$new $(DIST_DIR)/* --generate-notes --title "Release $$new"
