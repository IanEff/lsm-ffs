package sstable

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"

	"github.com/ianeff/lsm-ffs/internal/memtable"
)

type syncCloser interface {
	io.Closer
	Sync() error
}

type Writer struct {
	file syncCloser
	bw   *bufio.Writer
	buf  []byte
}

func NewWriter(file io.Writer) *Writer {
	return &Writer{
		bw:   bufio.NewWriter(file),
		file: file.(syncCloser),
		buf:  make([]byte, 0, 1024),
	}
}

func (w *Writer) Process(m *memtable.Memtable) error {
	i := m.Iterator()
	for i.HasNext() {
		key, entry := i.Next()
		err := w.writeDataBlock(key, entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeDataBlock(key, val []byte) error {
	keyLen, valLen := len(key), len(val)
	needed := 4 + keyLen + valLen
	buf := w.buf[:needed]
	binary.LittleEndian.PutUint16(buf[:], uint16(keyLen))
	binary.LittleEndian.PutUint16(buf[2:], uint16(valLen))
	copy(buf[4:], key)
	copy(buf[4+keyLen:], val)
	bytesWritten, err := w.bw.Write(buf)
	if err != nil {
		log.Fatal(err, bytesWritten)
		return err
	}
	return nil
}

func (w *Writer) Close() error {
	err := w.bw.Flush()
	if err != nil {
		return err
	}

	// Force OS to flush
	err = w.file.Sync()
	if err != nil {
		return err
	}

	w.bw = nil
	w.file = nil
	return err
}

var ErrKeyNotFound = errors.New("key not found")

type Reader struct {
	file io.Closer
	br   *bufio.Reader
	buf  []byte
}

func NewReader(file io.Reader) *Reader {
	r := &Reader{}
	r.file, _ = file.(io.Closer)
	r.br = bufio.NewReader(file)
	r.buf = make([]byte, 0, 1024)
	return r
}

func (r *Reader) Get(searchKey []byte) (memtable.Entry, error) {
	for {
		buf := r.buf[:4]
		_, err := io.ReadFull(r.br, buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		keyLen := binary.LittleEndian.Uint16(buf[:2])
		valLen := binary.LittleEndian.Uint16(buf[2:])
		needed := keyLen + valLen

		buf = r.buf[:needed]
		_, err = io.ReadFull(r.br, buf)
		if err != nil {
			return nil, err
		}
		key := buf[:keyLen]
		val := buf[keyLen:]

		if bytes.Compare(searchKey, key) == 0 {
			return val, nil
		}
	}
	return nil, ErrKeyNotFound
}

func (r *Reader) Close() error {
	err := r.file.Close()
	if err != nil {
		return err
	}
	r.file = nil
	r.br - nil
	return nil
}
