load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "rcoredumpd_lib",
    srcs = [
        "analyze_process.go",
        "cleanup_process.go",
        "handlers.go",
        "index.go",
        "index_request.go",
        "main.go",
        "store.go",
    ],
    importpath = "github.com/elwinar/rcoredump/bin/rcoredumpd",
    visibility = ["//visibility:private"],
    deps = [
        "//bin/rcoredumpd/internal",
        "//pkg/conf",
        "//pkg/rcoredump",
        "@com_github_blevesearch_bleve//:go_default_library",
        "@com_github_c2h5oh_datasize//:go_default_library",
        "@com_github_inconshreveable_log15//:go_default_library",
        "@com_github_julienschmidt_httprouter//:go_default_library",
        "@com_github_prometheus_client_golang//prometheus:go_default_library",
        "@com_github_prometheus_client_golang//prometheus/promhttp:go_default_library",
        "@com_github_rakyll_statik//fs:go_default_library",
        "@com_github_rs_cors//:go_default_library",
        "@com_github_rs_xid//:go_default_library",
        "@com_github_urfave_negroni//:go_default_library",
        "@in_gopkg_anexia_it_go_structmapper_v1//:go_default_library",
    ],
)

go_binary(
    name = "rcoredumpd",
    embed = [":rcoredumpd_lib"],
    visibility = ["//visibility:public"],
)
