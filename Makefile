BINARY := hr-cli
BIN_DIR := bin
MODULE  := github.com/TsCarpe/hr-cli
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build clean

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "-X $(MODULE)/cmd.Version=$(VERSION)" -o $(BIN_DIR)/$(BINARY) .

clean:
	rm -rf $(BIN_DIR)
