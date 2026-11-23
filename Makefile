PROTOS := $(wildcard proto/warpstack/v1/*.proto)
CLUSTER := deploy/as401144
PROVIDER ?= vultr

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

.PHONY: linux
linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/corticald-linux-amd64 ./cmd/corticald

.PHONY: deploy
deploy: linux
	@test -n "$(IP)" || (echo "usage: make deploy IP=1.2.3.4 [PROVIDER=vultr]" && exit 1)
	@test -f secrets.local || (echo "secrets.local is missing" && exit 1)
	ssh -o StrictHostKeyChecking=accept-new root@$(IP) 'mkdir -p /root/warpstack/zones && rm -f /root/warpstack/zones/*.zone'
	scp bin/corticald-linux-amd64 root@$(IP):/root/warpstack/corticald
	scp $(CLUSTER)/cluster.txtpb root@$(IP):/root/warpstack/
	scp $(CLUSTER)/zones/*.zone root@$(IP):/root/warpstack/zones/
	ssh root@$(IP) 'chmod +x /root/warpstack/corticald && /root/warpstack/corticald join --config /root/warpstack/cluster.txtpb --provider $(PROVIDER) --zones /root/warpstack/zones' < secrets.local

.PHONY: clean
clean:
	rm -rf bin