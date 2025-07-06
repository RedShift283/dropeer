package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Example configuration data (replace with your actual keys and IPs)
const (
	// Peer 1 (e.g., "Server")
	SERVER_PRIVATE_KEY = "4CuflX9D5WJOnoe+DPOY2hLd/RB5M61YX2yePS9C2WQ=" // Generate with Step 4
	SERVER_PUBLIC_KEY  = "SlqC6keE97J4uMLeaQGo/FmtZONMUnr3TrM0KQOVclI=" // Generate with Step 4
	SERVER_LISTEN_PORT = 51820
	SERVER_TUN_IP      = "10.0.0.1/24" // The IP for the server's end of the tunnel

	// Peer 2 (e.g., "Client")
	CLIENT_PRIVATE_KEY = "8DmqE9f+M5bA6CvReJ02o/rWmguHVOwjRUSvtHzn/Hs=" // Generate with Step 4
	CLIENT_PUBLIC_KEY  = "cKnvtgChojBioDPvY1Q7z6nVSV0zwkknx8iJtwzqNjU=" // Generate with Step 4
	CLIENT_LISTEN_PORT = 51821                                          // Client can listen on a different port if needed
	CLIENT_TUN_IP      = "10.0.0.2/24"                                  // The IP for the client's end of the tunnel

	// External endpoint of the server (e.g., public IP of the server machine)
	SERVER_EXTERNAL_IP = "127.0.0.1" // For local testing, use 127.0.0.1 or your host IP
)

// createServerConfig generates the WireGuard config for the server peer.
func createServerConfig() wgtypes.Config {
	serverPrivateKey, err := wgtypes.ParseKey(SERVER_PRIVATE_KEY)
	if err != nil {
		log.Fatalf("Failed to parse server private key: %v", err)
	}

	clientPublicKey, err := wgtypes.ParseKey(CLIENT_PUBLIC_KEY)
	if err != nil {
		log.Fatalf("Failed to parse client public key: %v", err)
	}

	_, clientTunIPNet, err := net.ParseCIDR(CLIENT_TUN_IP)
	if err != nil {
		log.Fatalf("Failed to parse client TUN IP: %v", err)
	}

	return wgtypes.Config{
		PrivateKey: &serverPrivateKey,
		ListenPort: &SERVER_LISTEN_PORT,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   clientPublicKey,
				AllowedIPs:                  []net.IPNet{*clientTunIPNet},          // Server allows client's tunnel IP
				PersistentKeepaliveInterval: wgtypes.NewDuration(15 * time.Second), // Optional
			},
		},
	}
}

// createClientConfig generates the WireGuard config for the client peer.
func createClientConfig() wgtypes.Config {
	clientPrivateKey, err := wgtypes.ParseKey(CLIENT_PRIVATE_KEY)
	if err != nil {
		log.Fatalf("Failed to parse client private key: %v", err)
	}

	serverPublicKey, err := wgtypes.ParseKey(SERVER_PUBLIC_KEY)
	if err != nil {
		log.Fatalf("Failed to parse server public key: %v", err)
	}

	// For a client, AllowedIPs often includes 0.0.0.0/0 to route all traffic
	// or the specific network you want to reach through the tunnel.
	// Here, we just allow the server's TUN IP.
	_, serverTunIPNet, err := net.ParseCIDR(SERVER_TUN_IP)
	if err != nil {
		log.Fatalf("Failed to parse server TUN IP: %v", err)
	}

	serverEndpointAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", SERVER_EXTERNAL_IP, SERVER_LISTEN_PORT))
	if err != nil {
		log.Fatalf("Failed to resolve server endpoint: %v", err)
	}

	return wgtypes.Config{
		PrivateKey: &clientPrivateKey,
		ListenPort: &CLIENT_LISTEN_PORT, // Client can listen on a different port
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   serverPublicKey,
				Endpoint:                    serverEndpointAddr,
				AllowedIPs:                  []net.IPNet{*serverTunIPNet, {IP: net.ParseIP("0.0.0.0"), Mask: net.IPv4Mask(0, 0, 0, 0)}}, // Allow server's tunnel IP and also 0.0.0.0/0 for full tunnel
				PersistentKeepaliveInterval: wgtypes.NewDuration(15 * time.Second),
			},
		},
	}
}
