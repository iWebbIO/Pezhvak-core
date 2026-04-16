package main

import (
	"fmt"
	"C" // Required for the c-shared buildmode
)

//export InitializeCore
func InitializeCore() {
	// This exported function makes the C-shared library valid for desktop GUI wrappers
	// (like Electron or Tauri) to call into Go.
}

func main() {
	// This is the entry point for the standalone CLI/Daemon executable
	fmt.Println("Pezhvak Core Daemon starting...")
	// TODO: Wire up local communication for Desktop GUIs.
}