.PHONY: build test clean run install uninstall

build:
	go build -o jotd ./cmd/agent

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f jotd

run: build
	./jotd

install: build
	sudo cp ./jotd /usr/local/bin/jotd
	./jotd install

uninstall: build
	./jotd uninstall
