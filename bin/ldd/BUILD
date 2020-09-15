load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "ldd_lib",
    srcs = ["main.go"],
    importpath = "github.com/elwinar/rcoredump/bin/ldd",
    visibility = ["//visibility:private"],
    deps = ["//pkg/elfx"],
)

go_binary(
    name = "ldd",
    embed = [":ldd_lib"],
    visibility = ["//visibility:public"],
)
