package core

import (
	"fmt"
	"time"

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
		// Set with a 48-hour TTL. In a revolution, messages older than 2 days 
		// without finding a path are likely stale or the recipient is compromised.
		e := badger.NewEntry(key, data).WithTTL(48 * time.Hour)
		return txn.SetEntry(e)
	})
}

func (s *BadgerStore) GetPending(peerID string) (map[string][]byte, error) {
	prefix := []byte(fmt.Sprintf("pending:%s:", peerID))
	pending := make(map[string][]byte)

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			// The key is only valid for the transaction, so we must copy it.
			msgID := string(key[len(prefix):])
			if err := item.Value(func(v []byte) error {
				// The value is only valid for the transaction, so we must copy it.
				valCopy := make([]byte, len(v))
				copy(valCopy, v)
				pending[msgID] = valCopy
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return pending, err
}

func (s *BadgerStore) DeletePending(peerID, messageID string) error {
	key := []byte(fmt.Sprintf("pending:%s:%s", peerID, messageID))
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (s *BadgerStore) HasMessage(messageID string) (bool, error) {
	found := false
	err := s.db.View(func(txn *badger.Txn) error {
		// We use a prefix scan because the message might be stored under 
		// any peerID prefix (pending:peerID:messageID)
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte("pending:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if item := it.Item(); item != nil && containsSuffix(item.Key(), messageID) {
				found = true
				return nil
			}
		}
		return nil
	})
	return found, err
}

func containsSuffix(key []byte, suffix string) bool {
	return len(key) >= len(suffix) && string(key[len(key)-len(suffix):]) == suffix
}

func (s *BadgerStore) Wipe() error {
	return s.db.DropAll()
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}