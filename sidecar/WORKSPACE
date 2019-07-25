workspace(name="cpp_sidecar")

# TODO: why are all of these nice rules deprecated ?
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository", "new_git_repository")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

git_repository(
    name = "io_opentracing_cpp",
    remote = "https://github.com/opentracing/opentracing-cpp.git",
    commit = "cf9b9d5c26ef985af2213521a4f0701b7e715db2", # use opentracing v1.5.1
)

## boost 1.70.0 ##

http_archive(
    name = "boost",
    urls = ["https://dl.bintray.com/boostorg/release/1.70.0/source/boost_1_70_0.tar.gz"],
    sha256="882b48708d211a5f48e60b0124cf5863c1534cd544ecd0664bb534a4b5d506e9",
    build_file="//bazel:boost.BUILD",
    strip_prefix="boost_1_70_0",
)

## for collector.proto ##

new_git_repository(
    name = "com_lightstep_tracer_common",
    remote = "https://github.com/lightstep/lightstep-tracer-common",
    commit = "bc2310a0474352fa2616bdc0a27457b146b136b4", # use tracer v0.9.0
    build_file = "//bazel:lightstep_tracer_common.BUILD"
)

## protobuf ##

# for cpp_proto_library rule
# https://github.com/stackb/rules_proto
http_archive(
    name = "build_stack_rules_proto",
    urls = ["https://github.com/stackb/rules_proto/archive/b93b544f851fdcd3fc5c3d47aee3b7ca158a8841.tar.gz"],
    sha256 = "c62f0b442e82a6152fcd5b1c0b7c4028233a9e314078952b6b04253421d56d61",
    strip_prefix = "rules_proto-b93b544f851fdcd3fc5c3d47aee3b7ca158a8841",
)
load("@build_stack_rules_proto//cpp:deps.bzl", "cpp_proto_library")
cpp_proto_library()


# collector.proto depends on some files in this repo
git_repository(
    name = "com_github_googleapis_googleapis",
    remote = "https://github.com/googleapis/googleapis",
    commit = "41d72d444fbe445f4da89e13be02078734fb7875",
)

# a dependency of @com_github_googleapis_googleapis
http_archive(
    name = "io_bazel_rules_go",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.17.0/rules_go-0.17.0.tar.gz"],
    sha256 = "492c3ac68ed9dcf527a07e6a1b2dcbf199c6bf8b35517951467ac32e421c06c1",
)
