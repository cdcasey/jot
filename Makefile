.PHONY: build test clean run

build:
	go build -o jot ./cmd/agent

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f jot

run: build
	./jot
