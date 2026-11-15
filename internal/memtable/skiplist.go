package memtable

import (
	"bytes"
	"math"
	"math/rand/v2"
)

const (
	// MaxHeight caps a node's tower; 16 levels suit about 65,536 keys at p = 1/2.
	MaxHeight = 16
	// PValue is the chance a node grows one more level.
	PValue = 0.5
)

// probabilities[h] is the chance, scaled to a uint32, that a node is taller than h.
var probabilities [MaxHeight]uint32

// node is one key in the list. tower[i] is the next node at level i.
type node struct {
	key   []byte
	entry Entry
	tower [MaxHeight]*node
}

// SkipList maps byte keys to entries, sorted by bytes.Compare. It stores keys as given, without copying
// them.
type SkipList struct {
	head   *node // sentinel before the first key; holds no key
	height int   // number of levels in use
}

func NewSkipList() *SkipList {
	sl := &SkipList{}
	sl.head = &node{}
	sl.height = 1
	return sl
}

// search returns key's node, or nil, plus the last node before key at each level, so Insert can link a
// new node in after them.
func (sl *SkipList) search(key []byte) (*node, [MaxHeight]*node) {
	var next *node
	var journey [MaxHeight]*node

	prev := sl.head
	for level := sl.height - 1; level >= 0; level-- {
		for next = prev.tower[level]; next != nil; next = prev.tower[level] {
			if bytes.Compare(key, next.key) <= 0 {
				break
			}
			prev = next
		}
		journey[level] = prev
	}

	if next != nil && bytes.Equal(key, next.key) {
		return next, journey
	}
	return nil, journey
}

// Find returns key's entry, and false if key is absent.
func (sl *SkipList) Find(key []byte) (Entry, bool) {
	found, _ := sl.search(key)
	if found == nil {
		return Entry{}, false
	}
	return found.entry, true
}

// Insert stores e under key, replacing any earlier entry.
func (sl *SkipList) Insert(key []byte, e Entry) {
	found, journey := sl.search(key)

	if found != nil {
		// update existing key
		found.entry = e
		return
	}

	height := randomHeight()
	nd := &node{key: key, entry: e}

	for level := 0; level < height; level++ {
		prev := journey[level]

		if prev == nil {
			// prev is nil if we're extendin the height of the tree
			prev = sl.head
		}
		nd.tower[level] = prev.tower[level]
		prev.tower[level] = nd
	}

	if height > sl.height {
		sl.height = height
	}
}

func (sl *SkipList) Delete(key []byte) bool {
	found, journey := sl.search(key)

	if found == nil {
		return false
	}

	for level := 0; level <= sl.height; level++ {
		if journey[level].tower[level] != found {
			break
		}
		journey[level].tower[level] = found.tower[level]
		found.tower[level] = nil
	}
	found = nil
	sl.shrink()

	return true
}

func (sl *SkipList) shrink() {
	for level := sl.height - 1; level >= 0; level-- {
		if sl.head.tower[level] == nil {
			sl.height--
		}
	}
}

func init() {
	probability := 1.0

	for level := 0; level < MaxHeight; level++ {
		probabilities[level] = uint32(probability * float64(math.MaxUint32))
		probability *= PValue
	}
}

// randomHeight picks a new node's height: it starts at 1, and each further level is added with
// probability PValue.
func randomHeight() int {
	seed := rand.Uint32()

	height := 1
	for height < MaxHeight && seed <= probabilities[height] {
		height++
	}

	return height
}
