BINARY  := pin
VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X punchlist/cmd.Version=$(VERSION)"

.PHONY: build test vet check install clean

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

check: test vet

install:
	go install $(LDFLAGS) .

clean:
	rm -f $(BINARY)
