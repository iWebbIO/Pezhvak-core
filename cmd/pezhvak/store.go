package core

// MessageStore defines the interface for store-and-forward persistence.
type MessageStore interface {
	SaveForLater(peerID, messageID string, data []byte) error
	GetPending(peerID string) (map[string][]byte, error)
	DeletePending(peerID, messageID string) error
	Wipe() error
	Close() error
}