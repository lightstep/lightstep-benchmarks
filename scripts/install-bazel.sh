# taken from:
# https://docs.bazel.build/versions/master/install-ubuntu.html#install-with-installer-ubuntu

# echo "deb [arch=amd64] https://storage.googleapis.com/bazel-apt stable jdk1.8" | sudo tee /etc/apt/sources.list.d/bazel.list
# curl https://bazel.build/bazel-release.pub.gpg | sudo apt-key add -
# sudo apt-get update && sudo apt-get install bazel



# NEED TO RUN THIS SCRIPT WITH SUDO

# this version is compatible with lightstep-cpp-tracer
BAZEL_VERSION="0.27.0"

# so we can install the official Oracle JDK
apt-get install software-properties-common
apt-get update
add-apt-repository -y ppa:webupd8team/java
apt-get update
apt-get install --no-install-recommends --no-install-suggests -y \
         wget \
         unzip \
         ca-certificates \
         openjdk-8-jdk # TODO: this is not installing...
wget https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
chmod +x bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
./bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
