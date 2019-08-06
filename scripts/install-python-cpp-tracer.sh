# install by cloning & building

git clone https://github.com/lightstep/lightstep-tracer-cpp.git
cd lightstep-tracer-cpp

BAZEL_OPTIONS="
startup --output_user_root=~/bazel-temp
build --incompatible_require_ctx_in_configure_features=false
build --incompatible_string_join_requires_strings=false
build --incompatible_use_python_toolchains=false"

echo "$BAZEL_OPTIONS" >> .bazelrc

sudo apt-get install cmake

sudo ./ci/do_ci.sh plugin

pip install /plugin/*.whl
