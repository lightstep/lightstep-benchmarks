package clientlib

var (
	allClients = map[string]TestClient{
		"cpp":    clientArgs("./cppclient"),
		"golang": clientArgs("./goclient"),
		"python": clientArgs("./pyclient.py"),
		"java":   clientArgs("java", "com.lightstep.benchmark.BenchmarkClient"),
		"nodejs": clientArgs("node", "--expose-gc", "--always_opt", "./jsclient.js"),
		"ruby":   clientArgs("ruby", "./benchmark.rb"),
	}
)
