.PHONY: build

build:
	mkdir -p bin
	go build -o bin/blinkcli ./cmd/blinkcli
