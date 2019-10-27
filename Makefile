root = $(shell pwd)
build_dir = $(root)/build
bin_dir = $(root)/bin

.PHONY: all
all: install build

.PHONY: install
install:
	npm install
	go mod download
	go get github.com/rakyll/statik

.PHONY: build
build: web rcoredumpd rcoredump monkey

.PHONY: web
web:
	npm run build
	statik -f -src build/web -dest ./bin/rcoredumpd/ -p internal

.PHONY: rcoredumpd
rcoredumpd:
	go build -o ${build_dir} ${bin_dir}/rcoredumpd

.PHONY: rcoredump
rcoredump:
	go build -o ${build_dir} ${bin_dir}/rcoredump

.PHONY: monkey
monkey:
	go build -o ${build_dir} ${bin_dir}/monkey
