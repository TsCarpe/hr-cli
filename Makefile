BINARY := hr-cli
BIN_DIR := bin

.PHONY: build clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) .

clean:
	rm -rf $(BIN_DIR)
