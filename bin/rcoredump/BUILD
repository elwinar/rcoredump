load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "rcoredump_lib",
    srcs = ["main.go"],
    importpath = "github.com/elwinar/rcoredump/bin/rcoredump",
    visibility = ["//visibility:private"],
    deps = [
        "//pkg/conf",
        "//pkg/rcoredump",
        "@com_github_inconshreveable_log15//:go_default_library",
    ],
)

go_binary(
    name = "rcoredump",
    embed = [":rcoredump_lib"],
    visibility = ["//visibility:public"],
)
