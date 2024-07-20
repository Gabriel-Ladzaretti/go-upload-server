# go-upload-server

A simple file upload server written in Go.

## Setup

### Build
 
Use the provided `Makefile` to build both components:

```shell
$ make
```
This will generate the `usrv` binary.

## Run

Start the server:

```shell
$ ./usrv
```

### Configuration

    -dir: Directory where files are saved (default: /tmp).
    -listen-addr: Address for the server to listen on, in the form "host:port". (default: :3000).
    -form-field: Form field name for file uploads (default: upload).
    -max-size: The maximum memory size (in megabytes) for storing part files in memory (default: 10).
    -read-timeout: Timeout for reading the request (default: 15s).
    -write-timeout: Timeout for writing the response (default: 15s).
    -idle-timeout: Timeout for keeping idle connections (default: 60s).


Example:

```shell
$ ./usrv -dir=/tmp/uploads -listen-addr=:5000 -form-field=upload -max-memory=20
http: 2024/07/20 19:13:37 Initialization completed successfully; Server config: Config{dir: /tmp/uploads, listenAddr: :5000, formUploadField: upload, maxFormFileSize: 20971520B, readTimeout: 15s, writeTimeout: 15s, idleTimeout: 1m0s}
http: 2024/07/20 19:13:37 listening on :5000
http: 2024/07/20 19:13:40 File uploaded successfully: wallhaven-nkxjw1_3440x1440.png
http: 2024/07/20 19:13:40 1721492020414658811 POST /upload [::1]:34466 curl/8.6.0
```

Send a file:
```shell
$ curl -v \
    -F upload=@/home/gbi/Downloads/wallhaven-nkxjw1_3440x1440.png \
    localhost:5000/upload
* Host localhost:5000 was resolved.
* IPv6: ::1
* IPv4: 127.0.0.1
*   Trying [::1]:5000...
* Connected to localhost (::1) port 5000
> POST /upload HTTP/1.1
> Host: localhost:5000
> User-Agent: curl/8.6.0
> Accept: */*
> Content-Length: 6469173
> Content-Type: multipart/form-data; boundary=------------------------QbWzHMHGMZqmIK7Tgz9HtP
> Expect: 100-continue
>
< HTTP/1.1 100 Continue
* We are completely uploaded and fine
< HTTP/1.1 200 OK
< X-Request-Id: 1721492020414658811
< Date: Sat, 20 Jul 2024 16:13:40 GMT
< Content-Length: 59
< Content-Type: text/plain; charset=utf-8
<
File uploaded successfully: wallhaven-nkxjw1_3440x1440.png
* Connection #0 to host localhost left intact
```

On the host:
```shell
$ la /tmp/uploads/
total 6.2M
-rw-r--r--. 1 gbi gbi 6.2M Jul 20 19:13 wallhaven-nkxjw1_3440x1440.png
```