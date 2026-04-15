package store

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	// Optimize for mobile platforms (uses less RAM by shrinking memtables)
	opts.WithMemTableSize(16 << 20)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) SaveForLater(peerID, messageID string, data []byte) error {
	key := []byte(fmt.Sprintf("pending:%s:%s", peerID, messageID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}

func (s *BadgerStore) GetPending(peerID string) ([][]byte, error) {
	var messages [][]byte
	prefix := []byte(fmt.Sprintf("pending:%s:", peerID))

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				valCopy := make([]byte, len(v))
				copy(valCopy, v)
				messages = append(messages, valCopy)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return messages, err
}

func (s *BadgerStore) DeletePending(peerID, messageID string) error {
	key := []byte(fmt.Sprintf("pending:%s:%s", peerID, messageID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}