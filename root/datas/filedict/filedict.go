package filedict

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"sync"
	"sync/atomic"
)

type FileDict[K comparable, V any] struct {
	data    map[K]V
	path    string
	encoder *json.Encoder
	file    *os.File
	mutex   sync.RWMutex
	closed  *atomic.Bool
}

var ErrIsClosed = errors.New("the FileDict is closed")

func New[K comparable, V any](path string, validate func(*K, *V) bool) (*FileDict[K, V], error) {
	table := FileDict[K, V]{
		data:   make(map[K]V),
		path:   path,
		closed: &atomic.Bool{},
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	table.file = file
	table.encoder = json.NewEncoder(file)
	if err := table.SyncRead(); err != nil {
		return nil, err
	}
	for k, v := range table.data {
		if !validate(&k, &v) {
			return nil, fmt.Errorf("invalidated entry: %v | %v", k, v)
		}
	}
	return &table, nil
}
func (table *FileDict[K, V]) Iter() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, value := range table.data {
			if !yield(key, value) {
				return
			}
		}
	}
}

// write contents to the file
func (table *FileDict[K, V]) SyncWrite() error {
	if err := table.lockOrClosed(); err != nil {
		return err
	}
	defer table.mutex.Unlock()
	// clear file
	if err := table.file.Truncate(0); err != nil {
		return err
	}
	if _, err := table.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return table.encoder.Encode(table.data)
}

// load contents from file. does nothing on eof
func (table *FileDict[K, V]) SyncRead() error {
	if err := table.lockOrClosed(); err != nil {
		return err
	}
	defer table.mutex.Unlock()
	//
	if err := json.NewDecoder(table.file).Decode(&table.data); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (table *FileDict[K, V]) IsClosed() bool {
	return table.closed.Load()
}

func (table *FileDict[K, V]) lockOrClosed() error {
	if table.IsClosed() {
		return ErrIsClosed
	}
	table.mutex.Lock()
	if table.IsClosed() {
		return ErrIsClosed
	}
	return nil
}

// permanent read-only state afterwards
func (table *FileDict[K, V]) Close() error {
	if err := table.lockOrClosed(); err != nil {
		return err
	}
	defer table.mutex.Unlock()
	if err := table.file.Close(); err != nil {
		return err
	}
	return nil
}
func (table *FileDict[K, V]) Get(key K) (value V, ok bool) {
	table.mutex.RLock()
	defer table.mutex.RUnlock()
	//
	value, ok = table.data[key]
	return
}
func (table *FileDict[K, V]) Set(key K, value V) error {
	if err := table.lockOrClosed(); err != nil {
		return err
	}
	defer table.mutex.Unlock()
	//
	table.data[key] = value
	return nil
}
