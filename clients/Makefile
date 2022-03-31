LD_FLAGS=-lgflags -llightstep_tracer -lopentracing -lprotobuf -levent
export CGO_CFLAGS=-Wno-undef-prefix

cpp_client: cpp_client.cpp
	g++ -O3 -pthread -std=c++11 -o cpp_client cpp_client.cpp ${LD_FLAGS}

go_client: go_client.go
	go build go_client.go
