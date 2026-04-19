package core

import (
	"fmt"
	"sync"
	"time"

	"pezhvak/internal/pb"
	"google.golang.org/protobuf/proto"
)

type messageBuffer struct {
	chunks     map[uint32][]byte
	total      uint32
	lastUpdate time.Time
}

type Router struct {
	mu         sync.Mutex
	assembler  map[string]*messageBuffer
	onComplete func(peerID string, messageID string, payload []byte)
	stopChan   chan struct{}
}

func NewRouter(callback func(string, string, []byte)) *Router {
	r := &Router{
		assembler:  make(map[string]*messageBuffer),
		onComplete: callback,
		stopChan:   make(chan struct{}),
	}
	go r.cleanupLoop()
	return r
}

func (r *Router) HandleIncomingPacket(peerID string, raw []byte) error {
	var packet pb.BLEPacket
	if err := proto.Unmarshal(raw, &packet); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	buf, exists := r.assembler[packet.MessageId]
	if !exists {
		// Limit reassembly buffer to 5000 chunks (~1MB) to prevent OOM
		if packet.TotalChunks > 5000 {
			return fmt.Errorf("message too large: %d chunks", packet.TotalChunks)
		}
		buf = &messageBuffer{
			chunks: make(map[uint32][]byte),
			total:  packet.TotalChunks,
		}
		r.assembler[packet.MessageId] = buf
	}

	buf.chunks[packet.ChunkIndex] = packet.PayloadChunk
	buf.lastUpdate = time.Now()

	if uint32(len(buf.chunks)) == buf.total {
		// Pre-calculate size to minimize allocations
		totalSize := 0
		for i := uint32(0); i < buf.total; i++ {
			totalSize += len(buf.chunks[i])
		}

		fullPayload := make([]byte, 0, totalSize)
		for i := uint32(0); i < buf.total; i++ {
			fullPayload = append(fullPayload, buf.chunks[i]...)
		}
		delete(r.assembler, packet.MessageId)
		go r.onComplete(peerID, packet.MessageId, fullPayload)
	}

	return nil
}

func (r *Router) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for id, buf := range r.assembler {
				// 60 second timeout for partial messages
				if now.Sub(buf.lastUpdate) > 60*time.Second {
					delete(r.assembler, id)
				}
			}
			r.mu.Unlock()
		case <-r.stopChan:
			return
		}
	}
}

func (r *Router) Stop() {
	close(r.stopChan)
	r.mu.Lock()
	defer r.mu.Unlock()
	// Clear memory on shutdown
	for k := range r.assembler {
		delete(r.assembler, k)
	}
}