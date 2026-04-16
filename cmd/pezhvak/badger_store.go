package core

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path).
		WithMemTableSize(16 << 20). // Optimize for mobile RAM
		WithCompactL0OnClose(true).
		WithLogger(nil).            // Prevent massive logcat spam on Android
		WithBypassLockGuard(true)   // Helps prevent file-lock crashes in strict iOS/Android sandboxes

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

func (s *BadgerStore) GetPending(peerID string, onMessage func(msgID string, data []byte)) error {
	prefix := []byte(fmt.Sprintf("pending:%s:", peerID))

	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			msgID := string(key[len(prefix):])
			err := item.Value(func(v []byte) error {
				valCopy := make([]byte, len(v))
				copy(valCopy, v)
				onMessage(msgID, valCopy)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *BadgerStore) DeletePending(peerID, messageID string) error {
	key := []byte(fmt.Sprintf("pending:%s:%s", peerID, messageID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}