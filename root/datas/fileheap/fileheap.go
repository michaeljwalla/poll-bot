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

const FILE_TIMEOUT time.Duration = 5000

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
		for _, item := range heap.inner.data {
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

func (heap *FileHeap[K]) Push(value K) error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
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

func (heap *FileHeap[K]) Pop() (K, error) {
	var zero K
	if err := heap.lockOrClosed(); err != nil {
		return zero, err
	}
	defer heap.mutex.Unlock()
	if heap.inner.Len() == 0 {
		return zero, nil
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

func (heap *FileHeap[K]) Merge(value ...K) error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
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

func (heap *FileHeap[K]) Peek() (K, bool) {
	heap.mutex.RLock()
	defer heap.mutex.RUnlock()
	var zero K
	if len(heap.inner.data) == 0 {
		return zero, false
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
	if i < 0 || i >= heap.Len() {
		return
	}
	return heap.inner.data[i], true
}
