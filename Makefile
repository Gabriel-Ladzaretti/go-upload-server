BIN_NAME := usrv

.PHONY: all build clean

all: build

build:
	go build -o $(BIN_NAME)

clean:
	rm -f $(BIN_NAME)
