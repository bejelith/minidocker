all: build

SRC=$(shell find . -iname *.go)
PBSOURCES=$(wildcard proto/*.proto)
BUILD=build
pb:
	mkdir -p pb

.PHONY: proto
proto:
	$(foreach F, $(PBSOURCES), \
		protoc -I proto --go_out=. --go-grpc_out=. --go_opt module=minidocker --go-grpc_opt=module=minidocker $(F);)

$(BUILD): $(SRC)
	@mkdir -p build
	GOBIN=$(PWD)/build go install ./cmd/...
	@touch $(BUILD)

TESTCACHE=$(BUILD)/.testcache
$(TESTCACHE): $(SRC) $(BUILD)
	CGO_ENABLED=1 go test -v -race ./...
	@touch $(TESTCACHE)

test: $(TESTCACHE)

install: build
	go install ./cmd/...

.PHONY: ca
ca: 
	$(MAKE) -C ./ca all
	touch 

clean:
	rm -r $(BUILD)