all: go_client otel_go_client js_client_v2

.PHONY: cpp_client
cpp_client:
	$(MAKE) -C clients cpp_client

.PHONY: go_client
go_client: 
	$(MAKE) -C clients go_client

.PHONY: otel_go_client
otel_go_client:
	$(MAKE) -C clients/otel_go_client build

.PHONY: js_client_v2
js_client_v2:
	$(MAKE) -C clients/js_client_v2 install

.PHONY: test
test:
	pytest test.py