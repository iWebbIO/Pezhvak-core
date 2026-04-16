package main

import (
	"C" // Required for the c-shared buildmode
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	core "pezhvak/cmd/pezhvak"
)

// desktopPlatform implements the NativePlatform interface for the CLI/Daemon
type desktopPlatform struct{}

func (p *desktopPlatform) SendBLE(peerID string, data []byte) error {
	fmt.Printf("[BLE TX] Sending %d bytes to %s\n", len(data), peerID)
	// TODO: Hook this up to a desktop Bluetooth library (like tinygo.org/x/bluetooth)
	return nil
}

func (p *desktopPlatform) OnMessageReceived(senderID string, plaintext []byte) {
	fmt.Printf("\n[MESSAGE RX] From %s: %s\n", senderID, string(plaintext))
}

//export InitializeCore
func InitializeCore() {
	// This exported function makes the C-shared library valid for desktop GUI wrappers
	// (like Electron or Tauri) to call into Go.
}

func main() {
	fmt.Println("Pezhvak Core Daemon starting...")

	// 1. Initialize the offline storage
	db, err := core.NewBadgerStore("./pezhvak-data")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// 2. Generate an ephemeral identity for the CLI (In production, you'd load/save this to disk)
	pub, priv, err := core.GenerateIdentity()
	if err != nil {
		log.Fatalf("Failed to generate keys: %v", err)
	}
	pubHex := hex.EncodeToString(pub[:])
	privHex := hex.EncodeToString(priv[:])
	fmt.Printf("My Node ID (Public Key): %s\n", pubHex)

	// 3. Instantiate the core logic
	platform := &desktopPlatform{}
	_, err = core.NewPezhvakCore(platform, db, privHex, pubHex)
	if err != nil {
		log.Fatalf("Failed to initialize Pezhvak Core: %v", err)
	}

	fmt.Println("Daemon is running and ready. Press Ctrl+C to exit.")

	// 4. Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down gracefully...")
}