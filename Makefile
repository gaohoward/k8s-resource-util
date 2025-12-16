.PHONY: generate

all: buildonly

generate:
	@echo "generate"
	protoc pkg/k8sservice/*.proto --go_out=pkg --go-grpc_out=pkg

buildonly: checkstatus
	go build -o out/resutil

build: generate buildonly

test: build
	go test ./...

checkstatus:
	@echo "Checking git status"
	@hack/check-git-status.sh
