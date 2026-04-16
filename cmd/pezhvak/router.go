package core

import (
	"sync"
	"time"

	"pezhvak/internal/pb"
	"google.golang.org/protobuf/proto"
)

// Router handles reassembly of incoming fragmented packets.
type Router struct {
	mu               sync.Mutex
	pendingAssembler map[string]*messageAssembler
	onMessage        func(peerID string, fullPayload []byte)
}

type messageAssembler struct {
	chunks      map[uint32][]byte
	totalChunks uint32
	lastUpdated time.Time
}

func NewRouter(onMessage func(peerID string, fullPayload []byte)) *Router {
	return &Router{
		pendingAssembler: make(map[string]*messageAssembler),
		onMessage:        onMessage,
	}
}

func (r *Router) HandleIncomingPacket(peerID string, rawPacket []byte) error {
	var packet pb.BLEPacket
	if err := proto.Unmarshal(rawPacket, &packet); err != nil {
		return err
	}

	r.mu.Lock()

	// MVP Cleanup: Evict incomplete messages older than 60 seconds
	now := time.Now()
	for id, asm := range r.pendingAssembler {
		if now.Sub(asm.lastUpdated) > time.Minute {
			delete(r.pendingAssembler, id)
		}
	}

	assembler, exists := r.pendingAssembler[packet.MessageId]
	if !exists {
		assembler = &messageAssembler{
			chunks:      make(map[uint32][]byte),
			totalChunks: packet.TotalChunks,
		}
		r.pendingAssembler[packet.MessageId] = assembler
	}

	assembler.lastUpdated = now
	assembler.chunks[packet.ChunkIndex] = packet.PayloadChunk

	var completePayload []byte
	if uint32(len(assembler.chunks)) == assembler.totalChunks {
		completePayload = make([]byte, 0)
		for i := uint32(0); i < assembler.totalChunks; i++ {
			completePayload = append(completePayload, assembler.chunks[i]...)
		}
		delete(r.pendingAssembler, packet.MessageId)
	}
	r.mu.Unlock() // Unlock before triggering FFI callbacks to prevent deadlocks

	if completePayload != nil && r.onMessage != nil {
		r.onMessage(peerID, completePayload)
	}

	return nil
}