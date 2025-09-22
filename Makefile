.PHONY: generate

all: build

generate:
	@echo "generate"
	protoc pkg/k8sservice/*.proto --go_out=pkg --go-grpc_out=pkg

buildonly:
	go build -o out/resutil

build: generate buildonly

test: build
	go test ./...

