.PHONY: build install run test lint fmt vet vulncheck validate clean rulesync-install

BIN := bin/slack-cli
CMD := ./cmd/slack-cli
INSTALL_DIR := /usr/local/bin

build:
	go build -o $(BIN) $(CMD)

install: build
	install -m 755 $(BIN) $(INSTALL_DIR)/slack-cli

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