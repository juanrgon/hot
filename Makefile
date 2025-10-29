BINARY := hot
BINDIR := $(CURDIR)/bin

.PHONY: build install run fmt test clean

build:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/$(BINARY) .

install:
	go install .

run:
	go run .

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

test:
	go test ./...

clean:
	rm -rf $(BINDIR)
