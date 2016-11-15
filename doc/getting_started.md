# LightStep Client-Libraries Benchmarks

## Getting Started

The client library benchmarks runs in a Linux environment and is provided with scripts to run locally, for development, or on Google Cloud VMs. 

### Quick Start

1. Set $GOPATH
2. Create a new git repository by cloning `git@github.com:lightstep/lightstep-benchmarks.git` into `${GOPATH}/src/github.com/lightstep`
3. DIR=${GOPATH}/src/github.com/lightstep/lightstep-benchmarks
3. Change into `${DIR}/scripts`
4. Update submodules `git submodule init`, `git submodule update`
5. Update `golang` dependencies with `go get`

### Run a single benchmark

The `benchmark.sh` script manages starting building and launching tests. The syntax is:

​	`BENCHMARK_VERBOSE=true ./benchmark.sh <testname> <clientname> <cpus> <configname>`

To run locally, set `cpus` to `local`. When running locally, `<testname>` is ignored, since local test results are not written to Cloud Storage. `<clientname>` is one of the supported clients, e.g., `golang`, `java`, `python`, etc. `<configname>` is the file base name of one of the benchmark configurations in `${DIR}/scripts/config`.

### Run a suite of benchmarks

The `launch_on_gcp.sh` script launches a suite of benchmarks on Google Cloud. The syntax is, e.g.,

​	`./launch_on_gcp.sh <command> <testname>`

where `<command>` is either `test` to start a new test or `logs` to review logs from an existing run, and `<testname>` is used to name the test results after the test completes. Several environment variables control the benchmarks, including:

​	`BENCHMARK_VERBOSE=true` causes the controller to print various logs

​	`BENCHMARK_PARAMS=<params>` sets the test parameters (see `${DIR}/params`)

​	`LANGUAGES="golang java"` allows running a subset of languages

​	`CONFIGS="one_thread_zero_logs"` allows running a subset of configurations

### Analyze a suite of benchmarks

The `summarize.go` program reads test results from storage and prints a summary to the console; in addition, it creates `gnuplot` scripts and generates performance graphs. The syntax is, e.g.,

​	`summarize —test=<testname>`.

### Adding a new client benchmark

TODO(jmacd):

1. Write the client program
2. Edit `${DIR}/scripts/docker/<client.sh>` to setup the docker build context
3. Edit `${DIR}/scripts/docker/Dockerfile.<client>` to build the docker image

### Caveats

The benchmark builds client programs using LightStep and OpenTracing dependencies from the source tree in some cases (e.g., Golang) and from the official package distribution source in others (e.g., Java, Python). Need to clarify the intention and make consistent across clients.

There are a number of TODOs!
