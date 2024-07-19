SRV_DIR := ./cmd/msrv
CLI_DIR := ./cmd/mcli
SRV_BIN := msrv
CLI_BIN := mcli

.PHONY: all build-msrv build-mcli clean

all: build-msrv build-mcli

build-msrv:
	go build -o $(SRV_BIN) $(SRV_DIR)

build-mcli:
	go build -o $(CLI_BIN) $(CLI_DIR)

clean:
	rm -f $(SRV_BIN) $(CLI_BIN)
