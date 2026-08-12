package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	instance  *sql.DB
	path      string
	open      *atomic.Bool
	timeoutMs time.Duration
}

type Record struct {
	row  *sql.Rows
	used *atomic.Bool
}

func New(path string, timeoutMs time.Duration) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", path)
	data := &SQLiteDB{
		instance:  db,
		path:      path,
		timeoutMs: timeoutMs,
		open:      &atomic.Bool{},
	}
	if err == nil {
		data.open.Store(true)
	}
	return data, err
}

func (db *SQLiteDB) IsOpen() bool {
	return db.open.Load()
}
func (db *SQLiteDB) Close() error {
	if !db.open.CompareAndSwap(true, false) {
		return errors.New("connection already closed")
	}
	return db.instance.Close()
}

func (db *SQLiteDB) Exec(query string, args ...any) (*sql.Result, error) {
	if !db.IsOpen() {
		return nil, errors.New("connection closed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), db.timeoutMs*time.Millisecond)
	defer cancel()
	//
	result, err := db.instance.ExecContext(ctx, query, args...)
	return &result, err
}

type Iter struct {
	data   *sql.Rows
	parent *SQLiteDB
	cancel context.CancelFunc
}

func (iter *Iter) NextScan(args ...any) (bool, error) {
	if !iter.parent.IsOpen() {
		return false, errors.New("connection closed")
	}
	if !iter.data.Next() {
		return false, iter.data.Err()
	}
	return true, iter.data.Scan(args...)
}

func (iter *Iter) Close() error {
	iter.cancel()
	return iter.data.Close()
}

func (db *SQLiteDB) Query(query string, args ...any) (*Iter, error) {
	if !db.IsOpen() {
		return nil, errors.New("connection closed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), db.timeoutMs*time.Millisecond)

	rows, err := db.instance.QueryContext(ctx, query, args...)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Iter{data: rows, parent: db, cancel: cancel}, nil
}
