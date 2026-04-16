package core

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"pezhvak/internal/pb"
	"google.golang.org/protobuf/proto"
)

const BLE_SAFE_PAYLOAD = 200

// PezhvakCore is the main struct exported to gomobile.
type PezhvakCore struct {
	platform NativePlatform
	store    MessageStore
	router   *Router
	privKey  *[32]byte
	pubKey   *[32]byte
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
	}
	copy(c.privKey[:], privBytes)
	copy(c.pubKey[:], pubBytes)

	c.router = NewRouter(func(peerID string, fullPayload []byte) {
		var msg pb.PezhvakMessage
		if err := proto.Unmarshal(fullPayload, &msg); err != nil {
			fmt.Println("Failed to unmarshal PezhvakMessage:", err)
			return
		}

		senderPubBytes, err := hex.DecodeString(msg.SenderId)
		if err != nil || len(senderPubBytes) != 32 {
			return // Invalid sender ID
		}

		var senderPub [32]byte
		copy(senderPub[:], senderPubBytes)

		plaintext, err := DecryptPayload(c.privKey, &senderPub, msg.EncryptedData)
		if err == nil {
			c.platform.OnMessageReceived(msg.SenderId, plaintext)
		}
	})
	return c, nil
}

func (c *PezhvakCore) ReceiveFromBLE(peerID string, rawPacket []byte) error {
	if len(rawPacket) == 0 {
		return nil
	}
	return c.router.HandleIncomingPacket(peerID, rawPacket)
}

func (c *PezhvakCore) FragmentAndSend(peerID string, messageID string, fullPayload []byte) error {
	totalLength := len(fullPayload)
	totalChunks := uint32(math.Ceil(float64(totalLength) / float64(BLE_SAFE_PAYLOAD)))

	for i := uint32(0); i < totalChunks; i++ {
		start := i * BLE_SAFE_PAYLOAD
		end := start + BLE_SAFE_PAYLOAD
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

		if err = c.platform.SendBLE(peerID, wireBytes); err != nil {
			_ = c.store.SaveForLater(peerID, messageID, fullPayload)
			return err
		}
	}
	return nil
}

// SendPlaintextMessage is called by the native Android/iOS UI to send a message to a peer.
func (c *PezhvakCore) SendPlaintextMessage(peerID string, recipientPubKeyHex string, plaintext []byte) error {
	recipientPubBytes, err := hex.DecodeString(recipientPubKeyHex)
	if err != nil || len(recipientPubBytes) != 32 {
		return errors.New("invalid recipient public key")
	}

	var recipientPub [32]byte
	copy(recipientPub[:], recipientPubBytes)

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

	msgID := fmt.Sprintf("%d-%s", msg.Timestamp, msg.SenderId[:8])
	return c.FragmentAndSend(peerID, msgID, wireBytes)
}

// SyncPendingMessages should be called by the native UI when a peer (re)connects.
// It retrieves all offline messages for the peer and attempts to send them.
func (c *PezhvakCore) SyncPendingMessages(peerID string) error {
	pending, err := c.store.GetPending(peerID)
	if err != nil {
		return err
	}

	for msgID, payload := range pending {
		if err := c.FragmentAndSend(peerID, msgID, payload); err == nil {
			_ = c.store.DeletePending(peerID, msgID)
		}
	}
	return nil
}