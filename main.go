package main

import (
	"fmt"
	"log"
	"os/exec"
)

// Ensure you have WireGuard installed on Windows and wg.exe is in your PATH.

func createWireGuardInterface(configPath string) error {
	cmd := exec.Command("wg", "quick", "up", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create WireGuard interface: %v\nOutput: %s", err, output)
	}
	fmt.Printf("WireGuard interface created:\n%s\n", output)
	return nil
}

func removeWireGuardInterface(configPath string) error {
	cmd := exec.Command("wg", "quick", "down", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove WireGuard interface: %v\nOutput: %s", err, output)
	}
	fmt.Printf("WireGuard interface removed:\n%s\n", output)
	return nil
}

func main() {
	configPath := "C:\\path\\to\\your\\wg0.conf" // Update this path

	err := createWireGuardInterface(configPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// To remove the interface, uncomment below:
	// err = removeWireGuardInterface(configPath)
	// if err != nil {
	//     log.Fatalf("Error: %v", err)
	// }
}
