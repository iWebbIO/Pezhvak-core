package core

// NativePlatform is implemented by the mobile/desktop UI layer.
type NativePlatform interface {
	SendBLE(peerID string, data []byte) error
	SetRadioPowerLevel(level int) error
	OnMessageReceived(senderID string, plaintext []byte)
}

// MessageStore is implemented by the BadgerStore engine.
type MessageStore interface {
	SaveForLater(peerID, messageID string, data []byte) error
	GetPending(peerID string) (map[string][]byte, error)
	DeletePending(peerID, messageID string) error
	
	MarkSeen(messageID string) error
	HasSeen(messageID string) (bool, error)
	
	Wipe() error
	Close() error
}