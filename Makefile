.PHONY: build install run test lint fmt vet vulncheck validate clean rulesync-install release build-all

BIN := bin/slack-cli
CMD := ./cmd/slack-cli
INSTALL_DIR := /usr/local/bin

build:
	go build -o $(BIN) $(CMD)

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

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

build-all:
	@rm -rf dist/
	@mkdir -p dist/
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		out="dist/slack-cli_$${os}_$${arch}"; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o $$out $(CMD); \
		tar -czf "$${out}.tar.gz" -C dist "slack-cli_$${os}_$${arch}"; \
		rm $$out; \
	done
	@echo "Archives written to dist/"

release:
	@prev=$$(sed -n 's/.*version = "\([^"]*\)".*/\1/p' cmd/slack-cli/main.go); \
	echo "Current version: $$prev"; \
	read -p "New version: " version; \
	sed -i '' "s/version = \"$$prev\"/version = \"$$version\"/" cmd/slack-cli/main.go; \
	git add cmd/slack-cli/main.go; \
	git commit -m "chore(release): bump version to v$$version"; \
	git tag -a "v$$version" -m "Release v$$version"; \
	git push origin main --tags; \
	$(MAKE) build-all; \
	gh release create "v$$version" dist/*.tar.gz \
		--title "v$$version" \
		--generate-notes
