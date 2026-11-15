# lsm-ffs

A barebones, concurrency-safe byte key-value store in Go, built as a log-structured merge tree and served over HTTP.

The store accepts byte slices and copies keys and values on writes; reads return a copy. The current map-backed memtable uses strings only as an internal adapter for byte keys, ready to be replaced by a skiplist. HTTP keys and values are URL-encoded, and successful reads return raw bytes as `application/octet-stream`.

WAL records are JSON with base64-encoded byte keys and values. Logs from the earlier string-based format require explicit migration; they cannot safely be read as the new format. Use a fresh data directory for disposable development data.

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
