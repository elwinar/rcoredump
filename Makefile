root = $(shell pwd)
build_dir = $(root)/build
bin_dir = $(root)/bin
ldflags="-X main.Version=`git describe --tags`"

.PHONY: help
help: ## Get help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: install build ## Install dependencies & build all targets

.PHONY: install
install: ## Install the dependencies needed for building the package
	npm install
	go mod download
	go get github.com/rakyll/statik
	go get github.com/karalabe/xgo

.PHONY: build
build: web rcoredumpd rcoredump monkey ## Build all targets

.PHONY: serve
serve: ## Run the web interface
	npm run serve

.PHONY: web
web: ## Build the web interface
	rm -rf ./build/web
	npm run build
	rm -rf ./bin/rcoredumpd/internal
	statik -f -src build/web -dest ./bin/rcoredumpd/ -p internal

.PHONY: rcoredumpd
rcoredumpd: ## Build the server
	go build -o ${build_dir} -ldflags $(ldflags) ${bin_dir}/rcoredumpd

.PHONY: rcoredump
rcoredump: ## Build the client
	go build -o ${build_dir} -ldflags $(ldflags) ${bin_dir}/rcoredump

.PHONY: monkey
monkey: ## Build the test crashers
	go build -o ${build_dir} ${bin_dir}/monkey-go
	gcc -o ${build_dir}/monkey-c ${bin_dir}/monkey-c/*.c

targets=linux/amd64,linux/386
pkg=github.com/elwinar/rcoredump/bin

.PHONY: release
release: ## Build the release files
	xgo --targets=$(targets) --ldflags=$(ldflags) $(pkg)/rcoredumpd
	xgo --targets=$(targets) --ldflags=$(ldflags) $(pkg)/rcoredump

