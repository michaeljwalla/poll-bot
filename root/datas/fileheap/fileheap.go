package fileheap

import (
	heaps "container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"sync"
	"sync/atomic"
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
	inner   fileHeapInner[K]
	path    string
	encoder *json.Encoder
	file    *os.File
	mutex   sync.RWMutex
	closed  *atomic.Bool
}

var ErrIsClosed = errors.New("the FileHeap is closed")

func New[K any](path string, less func(*K, *K) bool, validate func(*K) bool) (*FileHeap[K], error) {
	heap := FileHeap[K]{
		inner: fileHeapInner[K]{
			data: make([]K, 0),
			less: less,
		},
		path:   path,
		closed: &atomic.Bool{},
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	heap.file = file
	heap.encoder = json.NewEncoder(file)
	if err := heap.SyncRead(); err != nil {
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
	return nil
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
	return heaps.Pop(&heap.inner).(K), nil
}

func (heap *FileHeap[K]) Merge(value ...K) error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	heap.inner.data = append(heap.inner.data, value...)
	heaps.Init(&heap.inner)
	return nil
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

// write contents to the file
func (heap *FileHeap[K]) SyncWrite() error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	if err := heap.file.Truncate(0); err != nil {
		return err
	}
	if _, err := heap.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return heap.encoder.Encode(heap.inner.data)
}

// load contents from file. does nothing on eof
func (heap *FileHeap[K]) SyncRead() error {
	if err := heap.lockOrClosed(); err != nil {
		return err
	}
	defer heap.mutex.Unlock()
	if _, err := heap.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := json.NewDecoder(heap.file).Decode(&heap.inner.data); err != nil && err != io.EOF {
		return err
	}
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
