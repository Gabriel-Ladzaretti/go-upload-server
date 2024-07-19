# Multipart File Transfer

This project consists of two components:

- **Server (`msrv`)**: Receives multipart files and saves them to disk.
- **Client (`mcli`)**: Sends multipart files to the server.

## Setup

### Build
 
Use the provided `Makefile` to build both components:

```sh
$ make
```

This will generate the following binaries:

- `msrv` for the server
- `mcli` for the client

## Run

### Server:


```sh
$ ./msrv
```
### Client:

```sh
$ ./mcli -file <path-to-file>
```

### Configuration

    Server: Configure using command-line flags.
        -dir: Directory to save files (default: /tmp).
        -listen-addr: Port to listen on (default: :5000).
        -read-timeout, -write-timeout, -idle-timeout: Timeouts for server operations.

## Cleaning Up

To remove the built binaries:
```sh
make clean
```