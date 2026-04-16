package core

import (
	"errors"
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

const (
	assemblerTTL      = 60 * time.Second
	cleanupInterval = 5 * time.Minute
)

func NewRouter(onMessage func(peerID string, fullPayload []byte)) *Router {
	r := &Router{
		pendingAssembler: make(map[string]*messageAssembler),
		onMessage:        onMessage,
	}
	go r.cleanupStaleAssemblers()
	return r
}

func (r *Router) HandleIncomingPacket(peerID string, rawPacket []byte) error {
	var packet pb.BLEPacket
	if err := proto.Unmarshal(rawPacket, &packet); err != nil {
		return err
	}

	if packet.TotalChunks == 0 || packet.ChunkIndex >= packet.TotalChunks {
		return errors.New("invalid chunk index or total chunks")
	}

	r.mu.Lock()

	assembler, exists := r.pendingAssembler[packet.MessageId]
	if !exists {
		assembler = &messageAssembler{
			chunks:      make(map[uint32][]byte),
			totalChunks: packet.TotalChunks,
		}
		r.pendingAssembler[packet.MessageId] = assembler
	}

	assembler.lastUpdated = time.Now()
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

func (r *Router) cleanupStaleAssemblers() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		for id, asm := range r.pendingAssembler {
			if time.Since(asm.lastUpdated) > assemblerTTL {
				delete(r.pendingAssembler, id)
			}
		}
		r.mu.Unlock()
	}
}