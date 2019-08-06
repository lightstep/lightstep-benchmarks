# install by cloning & building

# git clone https://github.com/lightstep/lightstep-tracer-cpp.git
# cd lightstep-tracer-cpp
#
# sudo apt-get install cmake
#
# # bazel .24 required for this, but we are using a newer version
# bazel build //bridge/python:wheel.tgz \
#   --verbose_failures \
#   --incompatible_require_ctx_in_configure_features=false \
#   --incompatible_string_join_requires_strings=false \
#   --incompatible_use_python_toolchains=false
#
# cp bazel-genfiles/bridge/python/wheel.tgz .
# tar zxf wheel.tgz
# cp wheel/* lightstep_plugin/
# pip install /lightstep_plugin/*.whl

# install with pip (cheating, for now)
pip install lightstep-native
