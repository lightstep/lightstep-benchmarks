
# see: https://github.com/stackb/rules_proto
load("@build_stack_rules_proto//cpp:cpp_proto_library.bzl", "cpp_proto_library")

# proto_library(
#   name = "annotations_proto",
#   srcs = ["third_party/googleapis/google/api/annotations.proto"],
#
#   # deps = ["@com_google_protobuf//:any_proto"],
# )

proto_library(
  name = "collector_proto",
  srcs = ["collector.proto"],
  deps = [
    "@com_google_protobuf//:timestamp_proto",
    "@com_github_googleapis_googleapis//google/api:annotations_proto",
  ],
)

cpp_proto_library(
  name = "collector",
  visibility = ["//visibility:public"],
  deps = [":collector_proto"],
)
