package main

import (
	"C" // Required for the c-shared buildmode
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	core "pezhvak/cmd/pezhvak"
)

// desktopPlatform implements the NativePlatform interface for the CLI/Daemon
type desktopPlatform struct{}

type Config struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func (p *desktopPlatform) SendBLE(peerID string, data []byte) error {
	fmt.Printf("[BLE TX] Sending %d bytes to %s\n", len(data), peerID)
	// TODO: Hook this up to a desktop Bluetooth library (like tinygo.org/x/bluetooth)
	return nil
}

func (p *desktopPlatform) SetRadioPowerMode(boost bool) error {
	fmt.Printf("[RADIO] Power mode requested: boost=%v\n", boost)
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

func loadOrInitConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("No config file found. Generating new identity...")
		pub, priv, err := core.GenerateIdentity()
		if err != nil {
			return nil, fmt.Errorf("failed to generate keys: %w", err)
		}

		cfg := &Config{
			PublicKey:  hex.EncodeToString(pub[:]),
			PrivateKey: hex.EncodeToString(priv[:]),
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, fmt.Errorf("failed to write config file: %w", err)
		}
		fmt.Printf("Saved new identity to %s\n", path)
		return cfg, nil
	}

	fmt.Printf("Loading identity from %s\n", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

func main() {
	fmt.Println("Pezhvak Core Daemon starting...")
	dataDir := "./pezhvak-data"

	// Ensure the data directory exists before opening Badger or writing config
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// 1. Initialize the offline storage
	db, err := core.NewBadgerStore(dataDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 2. Load or generate node identity
	cfg, err := loadOrInitConfig(filepath.Join(dataDir, "identity.json"))
	if err != nil {
		log.Fatalf("Failed to manage identity: %v", err)
	}
	fmt.Printf("My Node ID (Public Key): %s\n", cfg.PublicKey)

	// 3. Instantiate the core logic
	platform := &desktopPlatform{}
	_, err = core.NewPezhvakCore(platform, db, cfg.PrivateKey, cfg.PublicKey)
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