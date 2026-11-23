PROTOS := $(wildcard proto/warpstack/v1/*.proto)

.PHONY: all
all: check build

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./...

.PHONY: check
check: vet test
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

.PHONY: proto-deps
proto-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

.PHONY: proto
proto: proto-deps
	PATH="$$(go env GOPATH)/bin:$$PATH" protoc -I=. \
	  --go_out=. --go_opt=paths=source_relative \
	  $(PROTOS)

.PHONY: build
build:
	go build -o bin/corticald ./cmd/corticald

.PHONY: clean
clean:
	rm -rf bin