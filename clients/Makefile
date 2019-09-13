LD_FLAGS=-lgflags -llightstep_tracer -lopentracing -lprotobuf -levent

cpp_client: cpp_client.cpp
	g++ -O3 -pthread -std=c++11 -o cpp_client cpp_client.cpp ${LD_FLAGS}
