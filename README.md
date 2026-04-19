# Pezhvak Core

Pezhvak Core is the decentralized mesh networking and cryptographic engine for the Pezhvak ecosystem. It is written in Go and designed to be compiled as a native library for mobile applications (Android/iOS) via `gomobile` and as a standalone daemon/shared library for desktop clients (Windows/macOS/Linux).

## Features

*   **Bluetooth Low Energy (BLE) Optimized:** Automatically fragments and reassembles large messages into BLE-safe packet chunks (<= 200 bytes).
*   **Store-and-Forward (Mesh Persistence):** Uses BadgerDB to temporarily queue messages for offline peers and syncs them once they reconnect.
*   **End-to-End Encryption:** Uses NaCl `box` (Curve25519, XSalsa20, and Poly1305) for authenticated public-key encryption. Intermediate mesh nodes cannot read your payloads.
*   **Adaptive Power Profiles:** Supports `Normal`, `High`, and `Max` radio modes to balance battery life against mesh throughput and range.
*   **Panic Wipe:** Instant local data destruction capability for high-risk environments.
*   **Cross-Platform Integration:** Exposes a clean `NativePlatform` interface so native Android (Kotlin), iOS (Swift), and Desktop clients can easily plug into their local Bluetooth hardware.

## Project Structure

*   `cmd/pezhvak/` - The core library package. Contains the router, offline store, crypto logic, and Gomobile exported entry points.
*   `cmd/pezhvak-cli/` - The standalone desktop daemon/CLI entry point.
*   `internal/pb/` - Auto-generated Protobuf structs (`schema.pb.go`) for serialization.
*   `cmd/pezhvak/schema.proto` - The Protocol Buffer definition for BLE packets and encrypted messages.

## Building the Project

### 1. Standalone Desktop Daemon
To run or build the desktop daemon (CLI) on your local machine:

```bash
# Run locally
go run ./cmd/pezhvak-cli

# Build executable
go build -o pezhvak-cli ./cmd/pezhvak-cli
```

### 2. Mobile Libraries (Android & iOS)
To build the `.aar` and `.xcframework` files, you need `gomobile` installed:

```bash
# Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init

# Build Android AAR
gomobile bind -target=android -androidapi 21 -o pezhvak.aar ./cmd/pezhvak

# Build iOS XCFramework
gomobile bind -target=ios -o pezhvak.xcframework ./cmd/pezhvak
```

## Continuous Integration

This repository includes a GitHub Actions workflow (`.github/workflows/release.yml`) that automatically compiles and releases all the necessary platform binaries:
*   Android `.aar`
*   iOS `.xcframework`
*   Desktop C-Shared Libraries (`.dll`, `.so`, `.dylib`)
*   Standalone Desktop Executables

You can trigger this workflow manually via the "Actions" tab or by pushing a version tag (e.g., `v1.0.0`) to the repository.