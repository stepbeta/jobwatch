.PHONY: ui build serve

ui:
	cd web && npm ci && npm run build

build: ui
	go build -tags ui -o jobwatch ./cmd/jobwatch

serve: build
	./jobwatch serve
