package env

import (
	"fmt"
	"os"
)

var (
	TestStorageBucket = GetEnv("BENCHMARK_BUCKET", "lightstep-client-benchmarks")
	TestTitle         = GetEnv("BENCHMARK_TITLE", "untitled")
	TestConfigName    = GetEnv("BENCHMARK_CONFIG_NAME", "unnamed")
	TestConfigFile    = GetEnv("BENCHMARK_CONFIG_FILE", "config.json")
	TestClient        = GetEnv("BENCHMARK_CLIENT", "unknown")
	TestZone          = GetEnv("BENCHMARK_ZONE", "")
	TestProject       = GetEnv("BENCHMARK_PROJECT", "")
	TestInstance      = GetEnv("BENCHMARK_INSTANCE", "")
	TestVerbose       = GetEnv("BENCHMARK_VERBOSE", "")
	TestParamsFile    = GetEnv("BENCHMARK_PARAMS_FILE", "")
)

func GetEnv(name, defval string) string {
	if r := os.Getenv(name); r != "" {
		return r
	}
	return defval
}

func Fatal(x ...interface{}) {
	panic(fmt.Sprintln(x...))
}

func Print(x ...interface{}) {
	if TestVerbose == "true" {
		fmt.Println(x...)
	}
}
