.PHONY: generate

all: build

generate:
	@echo "generate"
	protoc pkg/k8sservice/*.proto --go_out=pkg --go-grpc_out=pkg

build: generate
	go build -o out/resutil

test: build
	go test ./...

