.PHONY: ui build serve test test-go hooks

ui:
	cd web && npm ci && npm run build

build: ui
	go build -tags ui -o jobwatch ./cmd/jobwatch

serve: build
	./jobwatch serve

# Fast loop: Go checks only.
test-go:
	go vet ./...
	go test -count=1 ./...

# Local contract: the same Go checks plus the frontend suite.
test: test-go
	cd web && npm test

# Opt in to the committed hooks (required once per clone).
hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit .githooks/pre-push
	@echo "Git hooks installed from .githooks (bypass safely only with --no-verify; CI is the gate)."
