.PHONY: cpp_client
cpp_client:
	$(MAKE) -C clients cpp_client
.PHONY: go_client
go_client: 
	$(MAKE) -C clients go_client
