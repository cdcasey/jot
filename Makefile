.PHONY: build test clean run eval

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

eval:
	RUN_EVAL=1 go test ./eval/... -v -count=1 -timeout 300s
