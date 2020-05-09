root = $(shell pwd)
build_dir = $(root)/build
bin_dir = $(root)/bin
release_dir = $(root)/release

tag = $(shell git describe --tags --dirty)
commit = $(shell git rev-parse @)
built_at = $(shell date '+%FT%T%:z')
ldflags="-X main.Version=${tag} -X main.Commit=${commit} -X main.BuiltAt=${built_at}"

targets=linux/amd64
pkg=github.com/elwinar/rcoredump

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
build: web rcoredumpd rcoredump crashers ldd ## Build all targets

.PHONY: test
test: ## Run the package tests
	# We don't use the test shortcut ./... because the crashers would make
	# the command fail.
	go test ./pkg/... ./bin/rcoredump ./bin/rcoredumpd -race

.PHONY: serve
serve: ## Run the web interface
	./node_modules/.bin/parcel -d $(build_dir)/web/ --host 0.0.0.0 web/index.html web/favicon.svg

.PHONY: web
web: ## Build the web interface
	rm -rf $(build_dir)/web
	./node_modules/.bin/parcel build -d $(build_dir)/web/ web/index.js web/favicon.svg
	rm -rf $(bin_dir)/rcoredumpd/internal
	statik -f -src $(build_dir)/web -dest $(bin_dir)/rcoredumpd/ -p internal -m

.PHONY: rcoredumpd
rcoredumpd: ## Build the server
	go build -o $(build_dir) -ldflags $(ldflags) $(bin_dir)/rcoredumpd

.PHONY: rcoredump
rcoredump: ## Build the client
	go build -o $(build_dir) -ldflags $(ldflags) $(bin_dir)/rcoredump

.PHONY: ldd
ldd: ## Build the ldd tool
	go build -o $(build_dir) -ldflags $(ldflags) $(bin_dir)/ldd

.PHONY: crashers
crashers: ## Build the test crashers
	$(foreach crasher,$(shell ls bin/crashers/*.go), go build -o $(build_dir)/crashers/$(subst .,-,$(notdir $(crasher))) $(crasher);)
	$(foreach crasher,$(shell ls bin/crashers/*.c), gcc -o $(build_dir)/crashers/$(subst .,-,$(notdir $(crasher))) $(crasher);)

.PHONY: release
release: ## Build the release files
	mkdir -p $(release_dir)
	xgo --dest=$(release_dir) --targets=$(targets) --ldflags=$(ldflags) $(pkg)/bin/rcoredumpd
	xgo --dest=$(release_dir) --targets=$(targets) --ldflags=$(ldflags) $(pkg)/bin/rcoredump

