package lsm

const (
	memtableMaxBytes = 16
)

type Memtable struct {
	data map[string]entry
	size int
}

func NewMemtable(size int) *Memtable {
	return &Memtable{
		data: make(map[string]entry, size),
		size: size,
	}
}

type entry struct {
	value     string
	tombstone bool
}
