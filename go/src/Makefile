all:
	(cd ./github.com/lightstep/lightstep-tracer-javascript/; make build)
	(cd ./github.com/opentracing/opentracing-javascript/; make build)
	node ./node_modules/eslint/bin/eslint.js  *.js

test:
	go build goclient.go
	go build controller.go
	./controller --logtostderr -v=1 --client=golang --config ./test.json
