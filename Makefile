.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: proto-deps
proto-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

.PHONY: proto
proto: proto-deps
	protoc -I=. \
	  --go_out=. --go_opt=paths=source_relative \
	  proto/warpstack/v1/*.proto

.PHONY: clean
clean:
	rm -f proto/warpstack/v1/*.pb.go
	rm -rf bin

.PHONY: build
build: proto bin/corticald

bin/corticald:
	go build -o bin/corticald cmd/corticald/main.go