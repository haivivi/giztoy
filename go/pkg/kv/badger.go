package kv

import (
	"context"
	"errors"
	"iter"
	"log"

	badger "github.com/dgraph-io/badger/v4"
)

// Badger is a Store implementation backed by BadgerDB v4.
type Badger struct {
	db   *badger.DB
	opts *Options
}

// BadgerOptions configures the BadgerDB store.
type BadgerOptions struct {
	// Options is the common kv options (separator, etc.).
	Options *Options

	// Dir is the directory for BadgerDB data files.
	// Required.
	Dir string

	// InMemory runs BadgerDB in memory-only mode (no disk persistence).
	// Useful for testing with a real badger engine.
	InMemory bool

	// Logger sets the badger logger. If nil, badger's default logger is used.
	// Set to a no-op logger to silence badger output.
	Logger badger.Logger
}

// NewBadger creates a new BadgerDB-backed Store.
func NewBadger(bopts BadgerOptions) (*Badger, error) {
	if !bopts.InMemory && bopts.Dir == "" {
		return nil, errors.New("kv: BadgerOptions.Dir is required for on-disk mode")
	}
	dbOpts := badger.DefaultOptions(bopts.Dir)
	if bopts.InMemory {
		dbOpts = dbOpts.WithInMemory(true)
	}
	if bopts.Logger != nil {
		dbOpts = dbOpts.WithLogger(bopts.Logger)
	} else {
		dbOpts = dbOpts.WithLogger(defaultLogger{})
	}
	db, err := badger.Open(dbOpts)
	if err != nil {
		return nil, err
	}
	return &Badger{db: db, opts: bopts.Options}, nil
}

func (b *Badger) Get(_ context.Context, key Key) ([]byte, error) {
	k := b.opts.encode(key)
	var val []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	}
	return val, err
}

func (b *Badger) Set(_ context.Context, key Key, value []byte) error {
	k := b.opts.encode(key)
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(k, value)
	})
}

func (b *Badger) Delete(_ context.Context, key Key) error {
	k := b.opts.encode(key)
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(k)
	})
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil
	}
	return err
}

func (b *Badger) List(_ context.Context, prefix Key) iter.Seq2[Entry, error] {
	p := b.opts.encode(prefix)
	// Append separator so "a:b" prefix doesn't match "a:bc".
	var prefixBytes []byte
	if len(p) > 0 {
		prefixBytes = append(p, b.opts.sep())
	}

	return func(yield func(Entry, error) bool) {
		err := b.db.View(func(txn *badger.Txn) error {
			iterOpts := badger.DefaultIteratorOptions
			iterOpts.Prefix = prefixBytes
			it := txn.NewIterator(iterOpts)
			defer it.Close()

			for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
				item := it.Item()
				keyCopy := item.KeyCopy(nil)

				val, err := item.ValueCopy(nil)
				if err != nil {
					if !yield(Entry{}, err) {
						return nil
					}
					continue
				}

				entry := Entry{
					Key:   b.opts.decode(keyCopy),
					Value: val,
				}
				if !yield(entry, nil) {
					return nil
				}
			}
			return nil
		})
		if err != nil {
			yield(Entry{}, err)
		}
	}
}

func (b *Badger) BatchSet(_ context.Context, entries []Entry) error {
	wb := b.db.NewWriteBatch()
	defer wb.Cancel()
	for _, e := range entries {
		k := b.opts.encode(e.Key)
		if err := wb.Set(k, e.Value); err != nil {
			return err
		}
	}
	return wb.Flush()
}

func (b *Badger) BatchDelete(_ context.Context, keys []Key) error {
	wb := b.db.NewWriteBatch()
	defer wb.Cancel()
	for _, key := range keys {
		k := b.opts.encode(key)
		if err := wb.Delete(k); err != nil {
			return err
		}
	}
	return wb.Flush()
}

func (b *Badger) Close() error {
	return b.db.Close()
}

// defaultLogger wraps the standard log package for badger, suppressing
// debug and info level messages.
type defaultLogger struct{}

func (defaultLogger) Errorf(f string, v ...interface{}) { log.Printf("[badger] ERROR: "+f, v...) }
func (defaultLogger) Warningf(f string, v ...interface{}) {
	log.Printf("[badger] WARN: "+f, v...)
}
func (defaultLogger) Infof(string, ...interface{})  {}
func (defaultLogger) Debugf(string, ...interface{}) {}
