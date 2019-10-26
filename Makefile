root = $(shell pwd)
build_dir = $(root)/build
bin_dir = $(root)/bin
web_build_dir = $(root)/web/build

.PHONY: build
build: rcoredumpd rcoredump web monkey

.PHONY: web
web: web-dependencies
	cd web && npm run build && mv $(web_build_dir) $(build_dir)/public

web-dependencies: web/node_modules
	cd web && npm install

rcoredumpd:
	go build -o ${build_dir} ${bin_dir}/rcoredumpd

rcoredump:
	go build -o ${build_dir} ${bin_dir}/rcoredump

monkey:
	go build -o ${build_dir} ${bin_dir}/monkey
