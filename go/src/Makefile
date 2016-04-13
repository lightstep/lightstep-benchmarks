all:
	(cd ./github.com/lightstep/lightstep-tracer-javascript/; make build)
	(cd ./github.com/opentracing/opentracing-javascript/; make build)
	node ./node_modules/eslint/bin/eslint.js  *.js
