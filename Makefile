update:
	./scripts/update_clients.sh

smoketest:
	BENCHMARK_VERBOSE=true ./scripts/benchmark.sh test golang local test
