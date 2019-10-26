root = $(shell pwd)
build_dir = $(root)/build
bin_dir = $(root)/bin
web_build_dir = $(root)/web/build

.PHONY: build
build: rcoredumpd rcoredump web monkey

.PHONY: web
web:
	cd web && npm run build
	statik -f -src web/build/ -p public

web-dependencies:
	cd web && npm install

rcoredumpd:
	go build -o ${build_dir} ${bin_dir}/rcoredumpd

rcoredump:
	go build -o ${build_dir} ${bin_dir}/rcoredump

monkey:
	go build -o ${build_dir} ${bin_dir}/monkey
