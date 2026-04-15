package store

// MessageStore defines the interface for store-and-forward persistence.
type MessageStore interface {
	SaveForLater(peerID, messageID string, data []byte) error
	GetPending(peerID string) ([][]byte, error)
	DeletePending(messageID string) error
}