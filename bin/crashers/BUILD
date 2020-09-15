load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "crashers_lib",
    srcs = [
        "division-by-zero.go",
        "nil-pointer-exception.go",
        "panic.go",
    ],
    importpath = "github.com/elwinar/rcoredump/bin/crashers",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "crashers",
    embed = [":crashers_lib"],
    visibility = ["//visibility:public"],
)
