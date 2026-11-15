# lsm-ffs

The outlines of a log-structured merge tree backed key-value store, based
off of my initial [stringabase](https://github.com/IanEff/stringabase) Recurse Center application... uh... application.

A write-ahead log and a skiplist memtable, and that's the whole tree so far: no flush, no segments, no compaction. The shape is borrowed from Alex Chi Z's Rust [Mini-LSM course](https://skyzh.github.io/mini-lsm/week1-01-memtable.html#task-2-a-single-memtable-in-the-engine). Keys and values are bytes, copied in and copied out, and an empty value counts as present. Over HTTP they're URL-encoded, and reads come back raw as `application/octet-stream`.

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
