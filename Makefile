IMAGE ?= on-my-list
PORT ?= 8080
VOLUME ?= on-my-list-data

.DEFAULT_GOAL := help
.PHONY: help run test build docker-build docker-run clean

help: ## Show the available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

run: ## Run the server locally, with data in ./data
	PORT=$(PORT) DATA_DIR=data go run .

test: ## Run the tests
	go vet ./...
	go test ./...

build: ## Build the binary into ./bin
	go build -o bin/on-my-list .

docker-build: ## Build the Docker image
	docker build -t $(IMAGE) .

docker-run: docker-build ## Run the image, with data in a named volume
	docker run --rm -p $(PORT):8080 -v $(VOLUME):/data $(IMAGE)

clean: ## Remove build output
	rm -rf bin
