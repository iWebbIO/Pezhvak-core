package core

// NativePlatform is implemented by Kotlin (Android) or Swift (iOS).
type NativePlatform interface {
	// SendBLE pushes raw bytes down to the native BLE stack.
	SendBLE(peerID string, data []byte) error
	// SetRadioPowerLevel requests the native OS to adjust radio performance.
	// 0: Normal, 1: High, 2: Max (Full Performance + Continuous Scan)
	SetRadioPowerLevel(level int) error
	// OnMessageReceived passes a fully decrypted payload up to the native app.
	OnMessageReceived(senderID string, plaintext []byte)
}