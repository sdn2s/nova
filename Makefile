VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST    := dist/nova

.PHONY: test build dist sync clean

test:
	go test ./...
	GOOS=windows GOARCH=amd64 go vet ./...

build:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(DIST)/nova.exe ./cmd/nova

dist: test build
	rm -rf $(DIST)/bin $(DIST)/lists $(DIST)/strategies
	cp -R bin lists strategies $(DIST)/
	cd dist && rm -f nova-$(VERSION).zip && zip -qr nova-$(VERSION).zip nova
	@echo "dist/nova-$(VERSION).zip"

sync:
	scripts/sync-upstream.sh

clean:
	rm -rf dist
