package core

import (
	"sync"

	"pezhvak/internal/pb"
	"google.golang.org/protobuf/proto"
)

// Router handles reassembly of incoming fragmented packets.
type Router struct {
	mu               sync.Mutex
	pendingAssembler map[string]*messageAssembler
}

type messageAssembler struct {
	chunks      map[uint32][]byte
	totalChunks uint32
}

func NewRouter() *Router {
	return &Router{
		pendingAssembler: make(map[string]*messageAssembler),
	}
}

func (r *Router) HandleIncomingPacket(peerID string, rawPacket []byte) error {
	var packet pb.BLEPacket
	if err := proto.Unmarshal(rawPacket, &packet); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	assembler, exists := r.pendingAssembler[packet.MessageId]
	if !exists {
		assembler = &messageAssembler{
			chunks:      make(map[uint32][]byte),
			totalChunks: packet.TotalChunks,
		}
		r.pendingAssembler[packet.MessageId] = assembler
	}

	assembler.chunks[packet.ChunkIndex] = packet.PayloadChunk

	if uint32(len(assembler.chunks)) == assembler.totalChunks {
		fullPayload := make([]byte, 0)
		for i := uint32(0); i < assembler.totalChunks; i++ {
			fullPayload = append(fullPayload, assembler.chunks[i]...)
		}
		delete(r.pendingAssembler, packet.MessageId)
		// TODO: Pass the fullPayload to cryptographic validation/routing logic here
	}

	return nil
}