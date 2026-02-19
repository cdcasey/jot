.PHONY: build test clean run install uninstall

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

install: build
	./jot install

uninstall:
	jot uninstall
