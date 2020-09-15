load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "aux_lib",
    srcs = ["main.go"],
    importpath = "github.com/elwinar/rcoredump/bin/aux",
    visibility = ["//visibility:private"],
    deps = ["//pkg/auxv"],
)

go_binary(
    name = "aux",
    embed = [":aux_lib"],
    visibility = ["//visibility:public"],
)
