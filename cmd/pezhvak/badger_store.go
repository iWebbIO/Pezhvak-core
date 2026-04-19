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
		// Increased TTL to 72 hours (3 days) for better mesh resilience.
		e := badger.NewEntry(key, data).WithTTL(72 * time.Hour)
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
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			msgID := string(key[len(prefix):])

			// Optimized efficiency check: Use the current transaction to check sync status
			syncKey := []byte(fmt.Sprintf("sync:%s:%s", peerID, msgID))
			if _, err := txn.Get(syncKey); err == nil {
				continue
			}

			pending[msgID] = val
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

// MarkPeerSynced records that a message was successfully transmitted to a specific peer.
func (s *BadgerStore) MarkPeerSynced(peerID, messageID string) error {
	key := []byte(fmt.Sprintf("sync:%s:%s", peerID, messageID))
	return s.db.Update(func(txn *badger.Txn) error {
		// Sync records persist for 72h to match the message TTL.
		e := badger.NewEntry(key, []byte{1}).WithTTL(72 * time.Hour)
		return txn.SetEntry(e)
	})
}

// WasPeerSynced checks if we have already shared this message with the peer.
func (s *BadgerStore) WasPeerSynced(peerID, messageID string) (bool, error) {
	found := false
	err := s.db.View(func(txn *badger.Txn) error {
		key := []byte(fmt.Sprintf("sync:%s:%s", peerID, messageID))
		_, err := txn.Get(key)
		if err == nil {
			found = true
		} else if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
	return found, err
}

// MarkSeen creates a lightweight record that this message ID has been processed.
func (s *BadgerStore) MarkSeen(messageID string) error {
	key := []byte(fmt.Sprintf("seen:%s", messageID))
	return s.db.Update(func(txn *badger.Txn) error {
		// Seen records also have a TTL to prevent infinite growth.
		e := badger.NewEntry(key, []byte{1}).WithTTL(72 * time.Hour)
		return txn.SetEntry(e)
	})
}

// HasSeen provides O(1) deduplication check.
func (s *BadgerStore) HasSeen(messageID string) (bool, error) {
	found := false
	err := s.db.View(func(txn *badger.Txn) error {
		key := []byte(fmt.Sprintf("seen:%s", messageID))
		_, err := txn.Get(key)
		if err == nil {
			found = true
		} else if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
	return found, err
}

func (s *BadgerStore) Wipe() error {
	err := s.db.DropAll()
	if err != nil {
		return err
	}

	// Ensure the LSM tree is flattened to reclaim space from the OS immediately.
	// This is vital for mobile users with limited storage.
	if err := s.db.Flatten(1); err != nil {
		// Log or handle flattening error if necessary, though DropAll did the heavy lifting
	}

	// Run a value log GC after dropping to ensure space is reclaimed
	// immediately on storage-constrained mobile devices.
	err = s.db.RunValueLogGC(0.5)
	if err == badger.ErrNoRewrite {
		return nil
	}
	return err
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}