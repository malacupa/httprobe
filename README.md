# httprobe

Fork of awesome `github.com/tomnomnom/httprobe`. It changes mainly one thing for a different use-case which is you pass IP-ports CSV file (each line is IP,port1,port2,...) to stdin of httprobe and it scans those, doesn't try out standard HTTP/HTTPS ports as original httprobe.

## Install

```
▶ go get -u github.com/malacupa/httprobe
```

## Basic Usage

httprobe accepts line-delimited domains on `stdin`:

```
▶ cat recon/example/domains.txt
example.com:80
example.edu:8384
example.net:123
▶ cat recon/example/domains.txt | httprobe
http://example.com:80
http://example.net:123
https://example.edu:8384
```

## Concurrency

You can set the concurrency level with the `-c` flag:

```
▶ cat domains.txt | httprobe -c 50
```

## Timeout

You can change the timeout by using the `-t` flag and specifying a timeout in milliseconds:

```
▶ cat domains.txt | httprobe -t 20000
```

## Docker

Build the docker container:

```
▶ docker build -t httprobe .
```

Run the container, passing the contents of a file into stdin of the process inside the container. `-i` is required to correctly map `stdin` into the container and to the `httprobe` binary.

```
▶ cat domains.txt | docker run -i httprobe <args>
```

