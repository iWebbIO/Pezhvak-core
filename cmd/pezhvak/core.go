package core

import (
	"math"

	"pezhvak/internal/pb"
	"pezhvak/internal/store"
	"google.golang.org/protobuf/proto"
)

const BLE_SAFE_PAYLOAD = 200

// PezhvakCore is the main struct exported to gomobile.
type PezhvakCore struct {
	platform NativePlatform
	store    store.MessageStore
	router   *Router
}

func NewPezhvakCore(platform NativePlatform, db store.MessageStore) *PezhvakCore {
	return &PezhvakCore{
		platform: platform,
		store:    db,
		router:   NewRouter(),
	}
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