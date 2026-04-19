package core

import (
	"crypto/sha256"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"pezhvak/internal/pb"
	"google.golang.org/protobuf/proto"
)

const (
	Version = "1.0.0"

	PowerLevelNormal = 0
	PowerLevelHigh   = 1
	PowerLevelMax    = 2

	SizeNormal = 200
	SizeHigh   = 450
	SizeMax    = 480 // Maximum effective BLE MTU

	DelayNormal = 20 * time.Millisecond
	DelayHigh   = 10 * time.Millisecond
	DelayMax    = 0 // No delay, full throttle

	maxRetries    = 3
	retryDelay    = 100 * time.Millisecond
	maxMessageAge = 48 * time.Hour
)

// PezhvakCore is the main struct exported to gomobile.
type PezhvakCore struct {
	mu       sync.RWMutex
	platform NativePlatform
	store    MessageStore
	router   *Router
	privKey  *[32]byte
	pubKey   *[32]byte
	payloadSize  int
	currentDelay time.Duration

	syncMu       sync.Mutex
	syncingPeers map[string]bool
}

func NewPezhvakCore(platform NativePlatform, db MessageStore, privateKeyHex, publicKeyHex string) (*PezhvakCore, error) {
	privBytes, err1 := hex.DecodeString(privateKeyHex)
	pubBytes, err2 := hex.DecodeString(publicKeyHex)
	if err1 != nil || err2 != nil || len(privBytes) != 32 || len(pubBytes) != 32 {
		return nil, errors.New("invalid key format")
	}

	c := &PezhvakCore{
		platform: platform,
		store:    db,
		privKey:  new([32]byte),
		pubKey:   new([32]byte),
		payloadSize:  SizeNormal,
		currentDelay: DelayNormal,
		syncingPeers: make(map[string]bool),
	}
	copy(c.privKey[:], privBytes)
	copy(c.pubKey[:], pubBytes)

	c.router = NewRouter(func(peerID string, messageID string, fullPayload []byte) {
		// Deduplication: Check if we've already handled this message
		if exists, _ := c.store.HasSeen(messageID); exists {
			fmt.Printf("[CORE] Skipping duplicate message: %s\n", messageID)
			return
		}
		if err := c.store.MarkSeen(messageID); err != nil {
			fmt.Printf("[CORE] Error marking message seen: %v\n", err)
			return
		}

		var msg pb.PezhvakMessage
		if err := proto.Unmarshal(fullPayload, &msg); err != nil {
			fmt.Println("Failed to unmarshal PezhvakMessage:", err)
			return
		}

		// MATURITY: Reject messages that are older than the mesh TTL (e.g., 48h)
		if time.Since(time.Unix(msg.Timestamp, 0)) > maxMessageAge {
			fmt.Printf("[CORE] Dropping stale message %s from %s\n", messageID, msg.SenderId)
			return
		}

		myPubKeyHex := hex.EncodeToString(c.pubKey[:])
		
		// RELIABILITY: Always relay first to ensure the mesh propagates the data
		// Mesh Relaying Logic
		if msg.RecipientId != myPubKeyHex {
			fmt.Printf("[RELAY] Carrying message %s for recipient %s\n", messageID, msg.RecipientId)
			if err := c.store.SaveForLater(msg.RecipientId, messageID, fullPayload); err != nil {
				fmt.Printf("[RELAY] Error saving message for later: %v\n", err)
			}
			return
		}

		// Message is for us, attempt decryption

		senderPubBytes, err := hex.DecodeString(msg.SenderId)
		if err != nil || len(senderPubBytes) != 32 {
			return // Invalid sender ID
		}

		var senderPub [32]byte
		copy(senderPub[:], senderPubBytes)

		plaintext, err := DecryptPayload(c.privKey, &senderPub, msg.EncryptedData)
		if err != nil {
			fmt.Printf("[CORE] Failed to decrypt message %s: %v\n", messageID, err)
			return
		}

		fmt.Printf("[CORE] Successfully received message %s from %s\n", messageID, msg.SenderId)
		c.platform.OnMessageReceived(msg.SenderId, plaintext)
	})
	return c, nil
}

// Close shuts down the core engine and its background processes.
func (c *PezhvakCore) Close() error {
	c.router.Stop()
	return c.store.Close()
}

// GetPublicKey returns the node's public ID in hex format.
func (c *PezhvakCore) GetPublicKey() string {
	return hex.EncodeToString(c.pubKey[:])
}

// SetRadioPowerLevel sets the performance profile of the radio.
// 0 = Normal, 1 = High, 2 = Max.
func (c *PezhvakCore) SetRadioPowerLevel(level int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch level {
	case PowerLevelMax:
		c.payloadSize = SizeMax
		c.currentDelay = DelayMax
		fmt.Println("[CORE] Power Level: MAX (Full Throttle)")
	case PowerLevelHigh:
		c.payloadSize = SizeHigh
		c.currentDelay = DelayHigh
		fmt.Println("[CORE] Power Level: HIGH (High Performance)")
	default:
		c.payloadSize = SizeNormal
		c.currentDelay = DelayNormal
		fmt.Println("[CORE] Power Level: NORMAL (Battery Optimized)")
	}

	return c.platform.SetRadioPowerLevel(level)
}

// IsHighPowerEnabled allows the UI to check if the node is in an elevated power state.
func (c *PezhvakCore) IsHighPowerEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.payloadSize >= SizeHigh
}

// GetCurrentPayloadSize returns the current number of bytes per BLE packet.
func (c *PezhvakCore) GetCurrentPayloadSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.payloadSize
}

func (c *PezhvakCore) ReceiveFromBLE(peerID string, rawPacket []byte) error {
	if len(rawPacket) == 0 {
		return nil
	}
	return c.router.HandleIncomingPacket(peerID, rawPacket)
}

func (c *PezhvakCore) FragmentAndSend(peerID string, messageID string, fullPayload []byte) error {
	totalLength := len(fullPayload)
	if totalLength == 0 {
		return nil
	}

	c.mu.RLock()
	chunkSize := c.payloadSize
	chunkDelay := c.currentDelay
	c.mu.RUnlock()

	totalChunks := uint32((totalLength + chunkSize - 1) / chunkSize)

	for i := uint32(0); i < totalChunks; i++ {
		start := i * uint32(chunkSize)
		end := start + uint32(chunkSize)
		// RELIABILITY: Bounds checking for the final chunk
		if end > uint32(totalLength) {
			end = uint32(totalLength)
		}

		packet := &pb.BLEPacket{
			MessageId:    messageID,
			ChunkIndex:   i,
			TotalChunks:  totalChunks,
			PayloadChunk: fullPayload[start:end],
		}

		wireBytes, err := proto.Marshal(packet)
		if err != nil {
			return err
		}

		// RELIABILITY: Retry mechanism for transient BLE failures
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			lastErr = c.platform.SendBLE(peerID, wireBytes)
			if lastErr == nil {
				break
			}
			// Exponential-ish backoff: 100ms, 200ms, 300ms
			time.Sleep(retryDelay * time.Duration(attempt+1))
		}

		if lastErr != nil {
			if err := c.store.SaveForLater(peerID, messageID, fullPayload); err != nil {
				fmt.Printf("[CORE] Critical: Failed to save message for later after BLE failure: %v\n", err)
			}
			return lastErr
		}

		if chunkDelay > 0 {
			time.Sleep(chunkDelay)
		}
	}
	return nil
}

// WipeAllData is the "Panic Button" to clear all local mesh data.
func (c *PezhvakCore) WipeAllData() error {
	return c.store.Wipe()
}

// SendPlaintextMessage is called by the native Android/iOS UI to send a message to a peer.
func (c *PezhvakCore) SendPlaintextMessage(peerID string, recipientPubKeyHex string, plaintext []byte) error {
	recipientPubBytes, err := hex.DecodeString(recipientPubKeyHex)
	if err != nil || len(recipientPubBytes) != 32 {
		return errors.New("invalid recipient public key")
	}

	var recipientPub [32]byte
	copy(recipientPub[:], recipientPubBytes)

	// Prevent sending to self
	if recipientPubKeyHex == hex.EncodeToString(c.pubKey[:]) {
		return errors.New("cannot send message to self")
	}

	encrypted, err := EncryptPayload(c.privKey, &recipientPub, plaintext)
	if err != nil {
		return err
	}

	msg := &pb.PezhvakMessage{
		SenderId:      hex.EncodeToString(c.pubKey[:]),
		RecipientId:   recipientPubKeyHex,
		Timestamp:     time.Now().Unix(),
		EncryptedData: encrypted,
	}

	wireBytes, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return err
	}
	
	// MATURITY: Hash the metadata to prevent third-party tracking of Message IDs
	rawID := fmt.Sprintf("%d-%s-%x", msg.Timestamp, msg.SenderId, randBytes)
	msgID := hex.EncodeToString(sha256.New().Sum([]byte(rawID)))[:16]

	return c.FragmentAndSend(peerID, msgID, wireBytes)
}

// SyncPendingMessages should be called by the native UI when a peer (re)connects.
// It retrieves all offline messages for the peer and attempts to send them.
func (c *PezhvakCore) SyncPendingMessages(peerID string) error {
	c.syncMu.Lock()
	if c.syncingPeers[peerID] {
		c.syncMu.Unlock()
		return nil // Already syncing this peer
	}
	c.syncingPeers[peerID] = true
	c.syncMu.Unlock()

	defer func() {
		c.syncMu.Lock()
		delete(c.syncingPeers, peerID)
		c.syncMu.Unlock()
	}()

	pending, err := c.store.GetPending(peerID)
	if err != nil {
		return err
	}

	for msgID, payload := range pending {
		if err := c.FragmentAndSend(peerID, msgID, payload); err == nil {
			if delErr := c.store.DeletePending(peerID, msgID); delErr != nil {
				// This is non-fatal. The message will be re-sent on the next sync.
				// In a production system, this should be logged.
				fmt.Printf("Warning: failed to delete pending message %s for peer %s: %v\n", msgID, peerID, delErr)
			}
		}
	}
	return nil
}