.PHONY: gen
gen:
	@protoc -I proto \
		--go_out proto/gen \
		--go_opt paths=source_relative \
		--go-grpc_out proto/gen \
		--go-grpc_opt paths=source_relative \
		proto/helloworld.proto