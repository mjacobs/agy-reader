PKGS := ./...
BIN := bin/agy-reader
CMD := .
GOFILES := $(shell find . -maxdepth 1 -name '*.go') $(shell find internal -name '*.go')

.PHONY: fmt vet test race build check clean

fmt:
	gofmt -w $(GOFILES)

vet:
	go vet $(PKGS)

test:
	go test $(PKGS)

race:
	go test -race $(PKGS)

build:
	go build -o $(BIN) $(CMD)

check: fmt vet test build

clean:
	rm -rf bin
