package fileheap

import (
	heaps "container/heap"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"poll-bot/root/datas/sqlite"
	"sync"
	"sync/atomic"
	"time"
)

// inner type satisfies heap.Interface. Kept unexported so callers use
// FileHeap's typed Push/Pop instead of the any-typed plumbing.
type fileHeapInner[K any] struct {
	data []K
	less func(*K, *K) bool
}

func (h *fileHeapInner[K]) Len() int           { return len(h.data) }
func (h *fileHeapInner[K]) Less(i, j int) bool { return h.less(&h.data[i], &h.data[j]) }
func (h *fileHeapInner[K]) Swap(i, j int)      { h.data[i], h.data[j] = h.data[j], h.data[i] }
func (h *fileHeapInner[K]) Push(x any)         { h.data = append(h.data, x.(K)) }
func (h *fileHeapInner[K]) Pop() any {
	n := len(h.data) - 1
	v := h.data[n]
	h.data = h.data[:n]
	return v
}

// Just define less for your type
type FileHeap[K any] struct {
	inner  fileHeapInner[K]
	path   string
	file   *sqlite.SQLiteDB
	mutex  sync.RWMutex
	closed *atomic.Bool
}

var ErrIsClosed = errors.New("the FileHeap is closed")

const FILE_TIMEOUT time.Duration = 5000 * time.Millisecond

func newDBWithSchema(path string) (*sqlite.SQLiteDB, error) {
	db, err := sqlite.New(path, FILE_TIMEOUT)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS data (
		hash BLOB PRIMARY KEY,
		data BLOB NOT NULL
	);`)
	if err != nil {
		return nil, err
	}
	return db, nil
}
func New[K any](path string, less func(*K, *K) bool, validate func(*K) bool) (*FileHeap[K], error) {
	heap := FileHeap[K]{
		inner: fileHeapInner[K]{
			data: make([]K, 0),
			less: less,
		},
		path:   path,
		closed: &atomic.Bool{},
	}
	file, err := newDBWithSchema(path)
	if err != nil {
		return nil, err
	}
	heap.file = file
	if err := heap.read(); err != nil {
		return nil, err
	}
	for _, v := range heap.inner.data {
		if !validate(&v) {
			return nil, fmt.Errorf("invalidated entry: %v", v)
		}
	}
	heaps.Init(&heap.inner)
	return &heap, nil
}

func (heap *FileHeap[K]) Iter() iter.Seq[K] {
	return func(yield func(K) bool) {
		heap.mutex.RLock()
		snapshot := make([]K, len(heap.inner.data))
		copy(snapshot, heap.inner.data)
		heap.mutex.RUnlock()
		//
		for _, item := range snapshot {
			if !yield(item) {
				return
			}
		}
	}
}
func (heap *FileHeap[K]) IterLocked() iter.Seq[K] {
	return func(yield func(K) bool) {
		snapshot := make([]K, len(heap.inner.data))
		copy(snapshot, heap.inner.data)
		//
		for _, item := range snapshot {
			if !yield(item) {
				return
			}
		}
	}
}

func (heap *FileHeap[K]) Len() int {
	heap.mutex.RLock()
	defer heap.mutex.RUnlock()
	return heap.inner.Len()
}
func (heap *FileHeap[K]) LenLocked() int {
	return heap.inner.Len()

}

func (heap *FileHeap[K]) Lock() {
	heap.mutex.Lock()
}

func (heap *FileHeap[K]) Unlock() {
	heap.mutex.Unlock()
}

func (heap *FileHeap[K]) Push(value K) error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	return heap.PushLocked(value)
}
func (heap *FileHeap[K]) PushLocked(value K) error {
	heaps.Push(&heap.inner, value)
	jsonData, err := json.Marshal(&value)
	if err != nil {
		return fmt.Errorf("while marshaling %v: %v", value, err)
	}
	hash := sha256.Sum256(jsonData)
	_, err = heap.file.Exec(`REPLACE INTO data (hash, data) VALUES (?, ?);`, hash[:], jsonData)
	if err != nil {
		return fmt.Errorf("while inserting into DB %v: %v", value, err)
	}
	return err
}

// TryTake pops the top of the heap if want says to keep it. The predicate is
// evaluated on the top under the same write lock that does the pop, so nothing
// can push, pop or merge in between the decision and the take. want must not
// touch the heap itself; it would deadlock on the lock already held.
//
// top is nil when the heap is empty, and otherwise points at a copy of the top
// - the popped item when taken, the still-queued one when want declined it.
// A non-nil err means the item was taken but the file may still hold its row.
func (heap *FileHeap[K]) TryTake(want func(*K) bool) (top *K, taken bool, err error) {
	if err := heap.lockOrClosed(); err != nil {
		return nil, false, err
	}
	defer heap.mutex.Unlock()
	if heap.inner.Len() == 0 {
		return nil, false, nil
	}
	//copied so the caller never holds a pointer into the backing array
	value := heap.inner.data[0]
	if !want(&value) {
		return &value, false, nil
	}
	popped, err := heap.PopLocked()
	return &popped, true, err
}

func (heap *FileHeap[K]) Pop() (K, error) {
	var zero K
	if err := heap.lockOrClosed(); err != nil {
		return zero, err
	}
	defer heap.mutex.Unlock()
	return heap.PopLocked()
}

// caller holds the write lock and has already checked closed
func (heap *FileHeap[K]) PopLocked() (K, error) {
	var zero K
	if heap.inner.Len() == 0 {
		return zero, errors.New("empty")
	}
	//
	value := heaps.Pop(&heap.inner).(K)
	jsonData, err := json.Marshal(&value)
	if err != nil {
		return value, fmt.Errorf("while marshaling %v: %v", value, err)
	}
	hash := sha256.Sum256(jsonData)
	_, err = heap.file.Exec(`DELETE FROM data WHERE hash = ?;`, hash[:])
	return value, err
}

// ClearLocked empties the heap: both the backing rows and the in-memory data
// the rest of the type reads from. Dropping only the rows leaves every item
// still queued until the next process restart, so a caller clearing to rewrite
// the heap would append its survivors onto the full old contents. Caller holds
// the write lock. Memory is only cleared once the file agrees, so a failed
// delete leaves the heap as it was.
func (heap *FileHeap[K]) ClearLocked() error {
	if _, err := heap.file.Exec("DELETE FROM data;"); err != nil { //no TRUNCATE TABLE in sqlite
		return err
	}
	heap.inner.data = heap.inner.data[:0]
	return nil
}
func (heap *FileHeap[K]) Merge(value ...K) error {
	if len(value) == 0 { //otherwise builds a VALUES clause with no rows
		return nil
	}
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	return heap.MergeLocked(value...)
}
func (heap *FileHeap[K]) MergeLocked(value ...K) error {
	if len(value) == 0 {
		return nil
	}
	heap.inner.data = append(heap.inner.data, value...)
	heaps.Init(&heap.inner)

	data := make([]any, 0, len(value)*2)
	query := `REPLACE INTO data (hash, data) VALUES`
	for i, val := range value {
		jsonData, err := json.Marshal(&val)
		if err != nil {
			return fmt.Errorf("while marshaling %v: %v", value, err)
		}
		hash := sha256.Sum256(jsonData)
		data = append(data, hash[:], jsonData)
		//
		query += " (?, ?)"
		if i != len(value)-1 {
			query += ", "
		} else {
			query += ";"
		}
	}
	_, err := heap.file.Exec(query, data...)
	return err
}

// Peek returns a copy of the top. The copy is deliberate: a pointer into the
// backing array stays valid across mutations that reorder or replace whatever
// lives in that slot, so a caller holding one cannot tell whether it is still
// looking at the item it peeked. To act on the top, use TryTake - a Peek that
// a later call has to trust is exactly the race this type had.
func (heap *FileHeap[K]) Peek() (value K, ok bool) {
	heap.mutex.RLock()
	defer heap.mutex.RUnlock()
	if len(heap.inner.data) == 0 {
		return value, false
	}
	return heap.inner.data[0], true
}

// reset heap to file contents. call init after.
func (heap *FileHeap[K]) read() error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	//
	iter, err := heap.file.Query(`SELECT (data) FROM data;`)
	if err != nil {
		return err
	}
	defer iter.Close() //nolint

	heap.inner.data = heap.inner.data[:0]
	//loading from db
	for {
		var data []byte
		if ok, err := iter.NextScan(&data); err != nil {
			return err
		} else if !ok {
			break
		}
		//
		var next K
		if err := json.Unmarshal(data, &next); err != nil {
			return err
		}
		heap.inner.data = append(heap.inner.data, next)
	}
	//
	return nil
}

func (heap *FileHeap[K]) IsClosed() bool {
	return heap.closed.Load()
}

func (heap *FileHeap[K]) lockOrClosed() error {
	if heap.IsClosed() {
		return ErrIsClosed
	}
	heap.mutex.Lock()
	if heap.IsClosed() {
		heap.mutex.Unlock()
		return ErrIsClosed
	}
	return nil
}

// permanent read-only state afterwards
func (heap *FileHeap[K]) Close() error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	heap.closed.Store(true)
	return heap.file.Close()
}

// for manual traversing (though usually not very helpful)
func (heap *FileHeap[K]) Left(i int) (value K, ok bool) {
	return heap.At(2*i + 1)
}
func (heap *FileHeap[K]) Right(i int) (value K, ok bool) {
	return heap.At(2*i + 2)
}
func (heap *FileHeap[K]) Parent(i int) (value K, ok bool) {
	return heap.At((i - 1) / 2) //int div floored
}
func (heap *FileHeap[K]) At(i int) (value K, ok bool) {
	heap.mutex.RLock()
	defer heap.mutex.RUnlock()
	if i < 0 || i >= heap.inner.Len() {
		return
	}
	return heap.inner.data[i], true
}
