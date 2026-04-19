package main

import (
	"bufio"
	"C" // Required for the c-shared buildmode
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

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

func (p *desktopPlatform) SetRadioPowerLevel(level int) error {
	fmt.Printf("[RADIO] Power level adjusted to: %d\n", level)
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
	pCore, err := core.NewPezhvakCore(platform, db, cfg.PrivateKey, cfg.PublicKey)
	if err != nil {
		log.Fatalf("Failed to initialize Pezhvak Core: %v", err)
	}
	defer pCore.Close()

	fmt.Println("Daemon is running. Type '/help' for commands.")

	// Setup signal handling before the loop so the goroutine can access sigChan
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// 4. Start interactive command loop
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("> ")
		for scanner.Scan() {
			input := scanner.Text()
			if strings.HasPrefix(input, "/") {
				parts := strings.Split(input, " ")
				cmd := parts[0]

				switch cmd {
				case "/help":
					fmt.Println("Available Commands:")
					fmt.Println("  /send <recipient_pubkey> <message> - Send an encrypted message")
					fmt.Println("  /power <0|1|2>                   - Change radio power level")
					fmt.Println("  /wipe                             - PANIC: Wipe all data and identity")
					fmt.Println("  /quit                             - Exit the application")

				case "/send":
					if len(parts) < 3 {
						fmt.Println("Usage: /send <pubkey> <message>")
					} else {
						recipient := parts[1]
						msg := strings.Join(parts[2:], " ")
						// In CLI, we use a placeholder 'discovery' peer ID
						err := pCore.SendPlaintextMessage("desktop-peer-01", recipient, []byte(msg))
						if err != nil {
							fmt.Printf("Error sending: %v\n", err)
						} else {
							fmt.Println("Message passed to router...")
						}
					}

				case "/power":
					if len(parts) < 2 {
						fmt.Println("Usage: /power <0-2>")
					} else {
						level := parts[1]
						// Implementation for power level parsing and setting
						fmt.Printf("Setting power to %s\n", level)
					}

				case "/wipe":
					fmt.Println("PERFORMING PANIC WIPE...")
					pCore.WipeAllData()
					os.Remove(filepath.Join(dataDir, "identity.json"))
					fmt.Println("Data destroyed. Exiting.")
					os.Exit(0)

				case "/quit":
					// Send interrupt signal internally to trigger graceful shutdown
					sigChan <- os.Interrupt
				}
			}
			fmt.Print("> ")
		}
	}()

	// 5. Wait for interrupt signal (SIGINT or internal /quit) to gracefully shutdown
	<-sigChan

	fmt.Println("\nShutting down gracefully...")
}