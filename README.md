# lsm-ffs

A barebones, concurrency-safe string key-value store in Go, built as a log-structured merge tree and served over HTTP.

Structure loosely taken from "Cloud Native Go" by Matthew A. Titmus and "The Deeper Love of Go" by John Arundel, with a stab at commenting and design from the approach in John Ousterhout's "A Philosophy of Software Design."

## Usage
1. Start the server in one terminal:
```bash
go run ./cmd/lsm-ffs
```

2. In another terminal, store a value:
```bash
curl -i 'http://localhost:4000/set?somekey=somevalue'
```
You'll get an HTTP 200 status code and an empty response body.

3. Retrieve your value:
```bash
curl -w '\n' 'http://localhost:4000/get?key=somekey'
```
You'll get your value back.

4. Stop the server by hitting `ctrl-c` in its terminal.

## Layout

I've organized lsm-ffs into three packages, to keep concerns separate:

```
.
├── cmd/
│   └── lsm-ffs/        the composition root, where all defaults are baked in
└── internal/
    ├── server/         the HTTP server and endpoints
    └── lsm/            the key-value store: memtable and write-ahead log
```
