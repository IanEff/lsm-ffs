package lsm

type Memtable struct {
	data     map[string]entry
	bytes    int
	maxBytes int
}

func NewMemtable(maxBytes int) *Memtable {
	return &Memtable{data: make(map[string]entry), maxBytes: maxBytes}
}

func (m *Memtable) put(key string, e entry) {
	// Let's update the byte count
	if old, ok := m.data[key]; ok {
		m.bytes -= len(old.value)
	} else {
		m.bytes += len(key)
	}
	m.bytes += len(e.value)
	// ordered insert needs to go here
	m.data[key] = e
}

func (m *Memtable) full() bool {
	return m.bytes >= m.maxBytes
}

type entry struct {
	value     string
	tombstone bool
}
