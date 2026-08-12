package filedict

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"poll-bot/root/datas/sqlite"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type FileDict[K comparable, V any] struct {
	data   map[K]V
	path   string
	file   *sqlite.SQLiteDB
	mutex  sync.RWMutex
	closed *atomic.Bool
}

var ErrIsClosed = errors.New("the FileDict is closed")

const FILE_TIMEOUT_MS time.Duration = 5000

func newDBWithSchema(path string) (*sqlite.SQLiteDB, error) {
	db, err := sqlite.New(path, FILE_TIMEOUT_MS)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS data (
		key PRIMARY KEY,
		value BLOB NOT NULL
	);`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func New[K comparable, V any](path string, validate func(*K, *V) bool) (*FileDict[K, V], error) {
	table := FileDict[K, V]{
		data:   make(map[K]V),
		path:   path,
		closed: &atomic.Bool{},
	}
	file, err := newDBWithSchema(path)
	if err != nil {
		return nil, err
	}
	table.file = file
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

	if _, err := table.file.Exec(`DELETE FROM data;`); err != nil {
		return err
	}
	if len(table.data) == 0 {
		return nil
	}

	parts := make([]string, 0, len(table.data))
	args := make([]any, 0, len(table.data)*2)
	for k, v := range table.data {
		jsonData, err := json.Marshal(&v)
		if err != nil {
			return fmt.Errorf("While marshaling %v: %v", v, err)
		}
		parts = append(parts, "(?, ?)")
		args = append(args, k, jsonData)
	}
	query := "REPLACE INTO data (key, value) VALUES " + strings.Join(parts, ", ") + ";"
	_, err := table.file.Exec(query, args...)
	return err
}

// load contents from file
func (table *FileDict[K, V]) SyncRead() error {
	if err := table.lockOrClosed(); err != nil {
		return err
	}
	defer table.mutex.Unlock()

	iter, err := table.file.Query(`SELECT key, value FROM data;`)
	if err != nil {
		return err
	}
	defer iter.Close()

	table.data = make(map[K]V)
	for {
		var key K
		var raw []byte
		ok, err := iter.NextScan(&key, &raw)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		var value V
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		table.data[key] = value
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
		table.mutex.Unlock()
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
	table.closed.Store(true)
	return table.file.Close()
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
