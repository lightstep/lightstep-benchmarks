cc_binary(
    name = "sidecar",
    srcs = [
      "sidecar.cpp",
    ],
    deps = [
      "@boost//:boost_headers",
    ],
    # we need to explicitely link with pthread on Linux (but not OS X)
    # https://github.com/gflags/gflags/issues/176
    linkopts = [
      "-lpthread"
    ],
)
