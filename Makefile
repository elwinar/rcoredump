build_dir = ./build
bin_dir = ./bin

.PHONY: build
build:
	go build -o ${build_dir} ${bin_dir}/...

.PHONY: %
%:
	go build -o ${build_dir} ${bin_dir}/$*
