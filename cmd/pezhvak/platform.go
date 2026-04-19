package core

// NativePlatform is implemented by Kotlin (Android) or Swift (iOS).
type NativePlatform interface {
	// SendBLE pushes raw bytes down to the native BLE stack.
	SendBLE(peerID string, data []byte) error
	// SetRadioPowerMode requests the native OS to change TX power and MTU settings.
	SetRadioPowerMode(boost bool) error
	// OnMessageReceived passes a fully decrypted payload up to the native app.
	OnMessageReceived(senderID string, plaintext []byte)
}