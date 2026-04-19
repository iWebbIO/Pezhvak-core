package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"pezhvak/internal/pb"
	"google.golang.org/protobuf/proto"
)

// PezhvakCore is the main engine instance.
type PezhvakCore struct {
	platform NativePlatform
	store    MessageStore
	router   *Router
	privKey  *[32]byte
	pubKey   *[32]byte

	powerLevel int
}

// NewPezhvakCore initializes the engine with user identity and persistence.
func NewPezhvakCore(platform NativePlatform, store MessageStore, privHex, pubHex string) (*PezhvakCore, error) {
	privBytes, err := hex.DecodeString(privHex)
	if err != nil || len(privBytes) != 32 {
		return nil, fmt.Errorf("invalid private key")
	}
	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil || len(pubBytes) != 32 {
		return nil, fmt.Errorf("invalid public key")
	}

	var priv [32]byte
	var pub [32]byte
	copy(priv[:], privBytes)
	copy(pub[:], pubBytes)

	c := &PezhvakCore{
		platform:   platform,
		store:      store,
		privKey:    &priv,
		pubKey:     &pub,
		powerLevel: 0,
	}

	// Initialize router with the assembly completion callback
	c.router = NewRouter(c.onMessageAssembled)
	return c, nil
}

// onMessageAssembled is triggered when a multi-part message is fully reconstructed.
func (c *PezhvakCore) onMessageAssembled(peerID string, messageID string, payload []byte) {
	// 1. Deduplication check
	seen, err := c.store.HasSeen(messageID)
	if err != nil || seen {
		return
	}
	if err := c.store.MarkSeen(messageID); err != nil {
		// Log or handle error if needed, but we proceed to attempt decryption
	}

	var msg pb.PezhvakMessage
	if err := proto.Unmarshal(payload, &msg); err != nil {
		return
	}

	myID := hex.EncodeToString(c.pubKey[:])
	if msg.RecipientId == myID {
		// Scenario: It's for us. Decrypt and pass to UI.
		senderPubBytes, err := hex.DecodeString(msg.SenderId)
		if err != nil || len(senderPubBytes) != 32 {
			return
		}

		var senderPub [32]byte
		copy(senderPub[:], senderPubBytes)

		plaintext, err := DecryptPayload(c.privKey, &senderPub, msg.EncryptedData)
		if err == nil {
			c.platform.OnMessageReceived(msg.SenderId, plaintext)
		}
	} else {
		// Scenario: Relay (Mule logic). Store for 72 hours.
		_ = c.store.SaveForLater(msg.RecipientId, messageID, payload)
	}
}

// SendPlaintextMessage encrypts, fragments, and transmits a message.
func (c *PezhvakCore) SendPlaintextMessage(peerID string, recipientPubHex string, plaintext []byte) error {
	recipientBytes, err := hex.DecodeString(recipientPubHex)
	if err != nil || len(recipientBytes) != 32 {
		return fmt.Errorf("invalid recipient key")
	}
	var recipientPub [32]byte
	copy(recipientPub[:], recipientBytes)

	encrypted, err := EncryptPayload(c.privKey, &recipientPub, plaintext)
	if err != nil {
		return err
	}

	msg := &pb.PezhvakMessage{
		SenderId:      hex.EncodeToString(c.pubKey[:]),
		RecipientId:   recipientPubHex,
		Timestamp:     time.Now().Unix(),
		EncryptedData: encrypted,
	}

	payload, _ := proto.Marshal(msg)
	hash := sha256.Sum256(payload)
	msgID := hex.EncodeToString(hash[:])

	// Queue for later in case BLE fails or we encounter the recipient again
	_ = c.store.SaveForLater(recipientPubHex, msgID, payload)

	return c.fragmentAndSend(peerID, msgID, payload)
}

// ReceiveFromBLE feeds raw Bluetooth packets into the reassembly engine.
func (c *PezhvakCore) ReceiveFromBLE(peerID string, data []byte) error {
	return c.router.HandleIncomingPacket(peerID, data)
}

// SyncPendingMessages pushes all relevant stored messages to a newly connected peer.
func (c *PezhvakCore) SyncPendingMessages(peerID string) {
	pending, _ := c.store.GetPending(peerID)
	for msgID, payload := range pending {
		if err := c.fragmentAndSend(peerID, msgID, payload); err == nil {
			_ = c.store.MarkPeerSynced(peerID, msgID)
		}
	}
}

func (c *PezhvakCore) fragmentAndSend(peerID string, msgID string, payload []byte) error {
	mtu := c.getCurrentPayloadSize()
	totalChunks := uint32((len(payload) + mtu - 1) / mtu)

	for i := uint32(0); i < totalChunks; i++ {
		start := i * uint32(mtu)
		end := start + uint32(mtu)
		if end > uint32(len(payload)) {
			end = uint32(len(payload))
		}

		packet := &pb.BLEPacket{
			MessageId:    msgID,
			ChunkIndex:   i,
			TotalChunks:  totalChunks,
			PayloadChunk: payload[start:end],
		}

		packetBytes, _ := proto.Marshal(packet)
		if err := c.platform.SendBLE(peerID, packetBytes); err != nil {
			return err
		}

		if delay := c.getInterPacketDelay(); delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}

func (c *PezhvakCore) SetRadioPowerLevel(level int) {
	c.powerLevel = level
	_ = c.platform.SetRadioPowerLevel(level)
}

func (c *PezhvakCore) getCurrentPayloadSize() int {
	switch c.powerLevel {
	case 1: return 450
	case 2: return 480
	default: return 200
	}
}

func (c *PezhvakCore) getInterPacketDelay() time.Duration {
	switch c.powerLevel {
	case 1: return 10 * time.Millisecond
	case 2: return 0
	default: return 20 * time.Millisecond
	}
}

func (c *PezhvakCore) WipeAllData() error {
	c.router.Stop()
	return c.store.Wipe()
}

func (c *PezhvakCore) Close() error {
	c.router.Stop()
	return c.store.Close()
}